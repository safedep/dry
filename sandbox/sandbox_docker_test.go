package sandbox

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

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
			name:              "run command with sysbox-runc",
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

func TestDockerSandboxHealthCheck(t *testing.T) {
	if !isSandboxEndToEndTestEnabled() {
		t.Skip("Executor end-to-end tests are not enabled")
	}

	t.Run("successful health check with simple command", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{
			HealthCheckCommand: []string{"echo", "healthy"},
		})
		assert.NoError(t, err)

		// Verify sandbox is ready to execute commands
		r, err := sandbox.Execute(context.Background(), "echo", []string{"test"}, SandboxExecOpts{})
		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
	})

	t.Run("health check timeout", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true
		config.CreateWaitTimeout = 2 * time.Second

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		// This command will sleep longer than the timeout
		err = sandbox.Setup(context.Background(), SandboxSetupConfig{
			HealthCheckCommand: []string{"sh", "-c", "sleep 10 && exit 0"},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health check")
		assert.Contains(t, err.Error(), "timed out")
	})

	t.Run("health check failure with non-zero exit code", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true
		config.CreateWaitTimeout = 2 * time.Second

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		// This command will always fail
		err = sandbox.Setup(context.Background(), SandboxSetupConfig{
			HealthCheckCommand: []string{"sh", "-c", "exit 1"},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health check")
	})

	t.Run("health check with non-existent command", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true
		config.CreateWaitTimeout = 2 * time.Second

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{
			HealthCheckCommand: []string{"non-existent-command"},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health check")
	})

	t.Run("without health check command (existing behavior)", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		// Setup without health check - should work as before
		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		// Verify sandbox works normally
		r, err := sandbox.Execute(context.Background(), "echo", []string{"test"}, SandboxExecOpts{})
		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
	})

	t.Run("empty health check command array (existing behavior)", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		// Setup with empty health check array - should work as before
		err = sandbox.Setup(context.Background(), SandboxSetupConfig{
			HealthCheckCommand: []string{},
		})
		assert.NoError(t, err)

		// Verify sandbox works normally
		r, err := sandbox.Execute(context.Background(), "echo", []string{"test"}, SandboxExecOpts{})
		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
	})

	t.Run("health check with multiple arguments", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)

		defer sandbox.Close()

		// Test health check with command and multiple arguments
		err = sandbox.Setup(context.Background(), SandboxSetupConfig{
			HealthCheckCommand: []string{"sh", "-c", "test -d /tmp && echo 'healthy'"},
		})
		assert.NoError(t, err)

		// Verify sandbox is ready
		r, err := sandbox.Execute(context.Background(), "echo", []string{"test"}, SandboxExecOpts{})
		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
	})
}

func TestDockerSandboxExecuteWithIO(t *testing.T) {
	if !isSandboxEndToEndTestEnabled() {
		t.Skip("Executor end-to-end tests are not enabled")
	}

	t.Run("exec fails when IO is provided with skipWaitForCompletion", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)
		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		var stdout bytes.Buffer
		_, err = sandbox.Execute(context.Background(), "echo", []string{"hello world"}, SandboxExecOpts{
			Stdout:                &stdout,
			SkipWaitForCompletion: true,
		})

		assert.Error(t, err)
		assert.ErrorContains(t, err, "cannot skip completion")
	})

	t.Run("capture stdout from echo command", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)
		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		var stdout bytes.Buffer
		r, err := sandbox.Execute(context.Background(), "echo", []string{"hello world"}, SandboxExecOpts{
			Stdout: &stdout,
		})

		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
		assert.Equal(t, "hello world\n", stdout.String())
	})

	t.Run("provide stdin to cat command", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)
		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		input := "test input data\nmultiline\n"
		stdin := bytes.NewBufferString(input)
		var stdout bytes.Buffer

		r, err := sandbox.Execute(context.Background(), "cat", []string{}, SandboxExecOpts{
			Stdin:  stdin,
			Stdout: &stdout,
		})

		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
		assert.Equal(t, input, stdout.String())
	})

	t.Run("capture stderr from sh command", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)
		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		var stderr bytes.Buffer
		// sh -c 'echo "error message" >&2' writes to stderr
		r, err := sandbox.Execute(context.Background(), "sh", []string{"-c", "echo 'error message' >&2"}, SandboxExecOpts{
			Stderr: &stderr,
		})

		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
		assert.Equal(t, "error message\n", stderr.String())
	})

	t.Run("capture both stdout and stderr separately", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)
		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		var stdout, stderr bytes.Buffer
		// Command writes to both stdout and stderr
		r, err := sandbox.Execute(context.Background(), "sh", []string{"-c", "echo 'to stdout' && echo 'to stderr' >&2"}, SandboxExecOpts{
			Stdout: &stdout,
			Stderr: &stderr,
		})

		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
		assert.Equal(t, "to stdout\n", stdout.String())
		assert.Equal(t, "to stderr\n", stderr.String())
	})

	t.Run("stdin, stdout, and stderr all together", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)
		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		input := "input data"
		stdin := bytes.NewBufferString(input)
		var stdout, stderr bytes.Buffer

		// Read from stdin, echo to stdout, write to stderr
		r, err := sandbox.Execute(context.Background(), "sh", []string{"-c", "cat && echo 'stdout message' && echo 'stderr message' >&2"}, SandboxExecOpts{
			Stdin:  stdin,
			Stdout: &stdout,
			Stderr: &stderr,
		})

		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
		assert.Contains(t, stdout.String(), input)
		assert.Contains(t, stdout.String(), "stdout message")
		assert.Equal(t, "stderr message\n", stderr.String())
	})

	t.Run("only stdout capture without stderr", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)
		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		var stdout bytes.Buffer
		// Command writes to both stdout and stderr but we only capture stdout
		// In non-TTY mode (used when capturing output), stderr is separate and not captured
		r, err := sandbox.Execute(context.Background(), "sh", []string{"-c", "echo 'to stdout' && echo 'to stderr' >&2"}, SandboxExecOpts{
			Stdout: &stdout,
		})

		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
		// In non-TTY mode, only stdout is captured, stderr goes elsewhere
		output := stdout.String()
		assert.Equal(t, "to stdout\n", output)
	})

	t.Run("large output capture", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)
		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		var stdout bytes.Buffer
		// Generate a large output (1000 lines)
		r, err := sandbox.Execute(context.Background(), "sh", []string{"-c", "for i in $(seq 1 1000); do echo \"line $i\"; done"}, SandboxExecOpts{
			Stdout: &stdout,
		})

		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
		assert.Contains(t, stdout.String(), "line 1\n")
		assert.Contains(t, stdout.String(), "line 1000\n")
		// Verify we got approximately the right amount of data
		lines := bytes.Count(stdout.Bytes(), []byte("\n"))
		assert.Equal(t, 1000, lines)
	})

	t.Run("no IO streams provided - backward compatibility", func(t *testing.T) {
		config := DefaultDockerSandboxConfig(testDockerExecutorImage)
		config.Socket = getDockerSocketPath(t)
		config.PullImageIfMissing = true

		sandbox, err := NewDockerSandbox(config)
		assert.NoError(t, err)
		defer sandbox.Close()

		err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
		assert.NoError(t, err)

		// Execute without any IO streams - should work as before
		r, err := sandbox.Execute(context.Background(), "echo", []string{"hello"}, SandboxExecOpts{})

		assert.NoError(t, err)
		assert.Equal(t, 0, r.ExitCode)
	})
}
