package sandbox

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testDockerExecutorImage = "alpine:3.18.2"
)

func TestDockerSandboxExecute(t *testing.T) {
	cases := []struct {
		name                  string
		command               string
		args                  []string
		skipWaitForCompletion bool

		withCustomRuntime string // host runtime (default is runc)
		// Any of these exit codes are acceptable
		expectedExitCodes []int
		err               error
	}{
		{
			name:              "echo hello",
			command:           "echo",
			args:              []string{"hello"},
			expectedExitCodes: []int{0},
		},
		{
			name:              "run command with multiple args",
			command:           "ps",
			args:              []string{"-e", "-f"},
			withCustomRuntime: "sysbox-runc",
			expectedExitCodes: []int{0},
		},
		{
			name:              "cat a file",
			command:           "cat",
			args:              []string{"/etc/passwd"},
			expectedExitCodes: []int{0},
		},
		{
			name:              "non-existent command",
			command:           "non-existent",
			args:              []string{},
			withCustomRuntime: "sysbox-runc",
			expectedExitCodes: []int{1, 127, 126},
		},
		{
			name:              "bad arg to a command",
			command:           "cat",
			args:              []string{"/does-not-exist"},
			expectedExitCodes: []int{1},
		},
	}

	if !isSandboxEndToEndTestEnabled() {
		t.Skip("Executor end-to-end tests are not enabled")
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			config := DefaultDockerSandboxConfig(testDockerExecutorImage)
			config.Socket = getDockerSocketPath(t)
			config.PullImageIfMissing = true

			if c.withCustomRuntime != "" {
				config.Runtime = c.withCustomRuntime
			}

			sandbox, err := NewDockerSandbox(config)
			assert.NoError(t, err)

			defer sandbox.Close()

			err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
			assert.NoError(t, err)

			r, err := sandbox.Execute(context.Background(), c.command, c.args, SandboxExecOpts{
				SkipWaitForCompletion: c.skipWaitForCompletion,
			})

			if c.err != nil {
				assert.Error(t, err)
				assert.Equal(t, c.err, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, c.expectedExitCodes, r.ExitCode)
			}
		})
	}
}

func TestDockerSandboxSetup(t *testing.T) {
	if !isSandboxEndToEndTestEnabled() {
		t.Skip("Executor end-to-end tests are not enabled")
	}

	t.Run("when socket path is wrong", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = "/does-not-exist"
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NotNil(t, sandbox)
		assert.NoError(t, err)

		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Cannot connect to the Docker daemon")
	})

	t.Run("when the image is not present and pull is enabled", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)
	})

	t.Run("when the image already exists it should not be pulled again", func(t *testing.T) {
		// First setup to make sure the image exists
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		err = sandbox.Close()
		assert.NoError(t, err)

		config.PullImageIfMissing = false
		secondSandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		err = secondSandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		r, err := secondSandbox.Execute(context.Background(), "echo", []string{"hello"}, SandboxExecOpts{})
		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)

		err = secondSandbox.Close()
		assert.NoError(t, err)
	})
}

func TestDockerSandboxWithEnvironmentVariables(t *testing.T) {
	if !isSandboxEndToEndTestEnabled() {
		t.Skip("Executor end-to-end tests are not enabled")
	}

	t.Run("when environment variables are set", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		envVars := map[string]string{
			"ENV_VAR_1": "value1",
			"ENV_VAR_2": "value2",
		}

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{
			EnvironmentVariables: envVars,
		})
		assert.NoError(t, err)

		r, err := sandbox.Execute(context.Background(), "sh", []string{"-c", "env > /tmp/env.txt"}, SandboxExecOpts{})
		assert.NoError(t, err)
		assert.Zero(t, r.ExitCode)

		output, err := sandbox.ReadFile(context.Background(), "/tmp/env.txt")
		assert.NoError(t, err)
		assert.NotNil(t, output)

		content, err := io.ReadAll(output)
		assert.NoError(t, err)

		assert.Contains(t, string(content), "ENV_VAR_1=value1")
		assert.Contains(t, string(content), "ENV_VAR_2=value2")
		assert.NotContains(t, string(content), "ENV_VAR_3=value3")
	})
}

func TestDockerSandboxReadWriteFile(t *testing.T) {
	if !isSandboxEndToEndTestEnabled() {
		t.Skip("Executor end-to-end tests are not enabled")
	}

	t.Run("when file is not present", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		_, err = sandbox.ReadFile(context.Background(), "/tmp/test.txt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Could not find the file /tmp/test.txt in container ")
	})

	t.Run("when file is present", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		err = sandbox.WriteFile(context.Background(), "/tmp/test.txt", bytes.NewReader([]byte("test")))
		assert.NoError(t, err)

		output, err := sandbox.ReadFile(context.Background(), "/tmp/test.txt")
		assert.NoError(t, err)
		assert.NotNil(t, output)

		content, err := io.ReadAll(output)
		assert.NoError(t, err)
		assert.Equal(t, "test", string(content))
	})
}
