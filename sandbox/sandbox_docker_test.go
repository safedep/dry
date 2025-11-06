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

	cases := []struct {
		name                  string
		command               string
		args                  []string
		stdinData             string
		skipWaitForCompletion bool
		attachStdout          bool // Force attach stdout even without expected output
		attachStderr          bool // Force attach stderr even without expected output
		expectError           bool
		errorContains         string
		expectedExitCode      int
		expectedStdout        string
		expectedStderr        string
		stdoutContains        []string
		stderrContains        []string
		validateFn            func(t *testing.T, stdout, stderr string)
	}{
		{
			name:                  "exec fails when IO is provided with skipWaitForCompletion",
			command:               "echo",
			args:                  []string{"hello world"},
			attachStdout:          true,
			skipWaitForCompletion: true,
			expectError:           true,
			errorContains:         "cannot skip completion",
		},
		{
			name:             "capture stdout from echo command",
			command:          "echo",
			args:             []string{"hello world"},
			expectedExitCode: 0,
			expectedStdout:   "hello world\n",
		},
		{
			name:             "provide stdin to cat command",
			command:          "cat",
			args:             []string{},
			stdinData:        "test input data\nmultiline\n",
			expectedExitCode: 0,
			expectedStdout:   "test input data\nmultiline\n",
		},
		{
			name:             "capture stderr from sh command",
			command:          "sh",
			args:             []string{"-c", "echo 'error message' >&2"},
			expectedExitCode: 0,
			expectedStderr:   "error message\n",
		},
		{
			name:             "capture both stdout and stderr separately",
			command:          "sh",
			args:             []string{"-c", "echo 'to stdout' && echo 'to stderr' >&2"},
			expectedExitCode: 0,
			expectedStdout:   "to stdout\n",
			expectedStderr:   "to stderr\n",
		},
		{
			name:             "stdin, stdout, and stderr all together",
			command:          "sh",
			args:             []string{"-c", "cat && echo 'stdout message' && echo 'stderr message' >&2"},
			stdinData:        "input data",
			expectedExitCode: 0,
			stdoutContains:   []string{"input data", "stdout message"},
			expectedStderr:   "stderr message\n",
		},
		{
			name:             "only stdout capture without stderr",
			command:          "sh",
			args:             []string{"-c", "echo 'to stdout' && echo 'to stderr' >&2"},
			expectedExitCode: 0,
			expectedStdout:   "to stdout\n",
		},
		{
			name:             "large output capture",
			command:          "sh",
			args:             []string{"-c", "for i in $(seq 1 1000); do echo \"line $i\"; done"},
			expectedExitCode: 0,
			validateFn: func(t *testing.T, stdout, stderr string) {
				assert.Contains(t, stdout, "line 1\n")
				assert.Contains(t, stdout, "line 1000\n")
				lines := bytes.Count([]byte(stdout), []byte("\n"))
				assert.Equal(t, 1000, lines)
			},
		},
		{
			name:             "no IO streams provided - backward compatibility",
			command:          "echo",
			args:             []string{"hello"},
			expectedExitCode: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultDockerSandboxConfig(testDockerExecutorImage)
			config.Socket = getDockerSocketPath(t)
			config.PullImageIfMissing = true

			sandbox, err := NewDockerSandbox(config)
			assert.NoError(t, err)
			defer sandbox.Close()

			err = sandbox.Setup(context.Background(), SandboxSetupConfig{})
			assert.NoError(t, err)

			var stdin io.Reader
			if tc.stdinData != "" {
				stdin = bytes.NewBufferString(tc.stdinData)
			}

			var stdout, stderr bytes.Buffer
			opts := SandboxExecOpts{
				SkipWaitForCompletion: tc.skipWaitForCompletion,
			}

			if tc.stdinData != "" {
				opts.Stdin = stdin
			}

			if tc.attachStdout || tc.expectedStdout != "" || len(tc.stdoutContains) > 0 || tc.validateFn != nil {
				opts.Stdout = &stdout
			}

			if tc.attachStderr || tc.expectedStderr != "" || len(tc.stderrContains) > 0 {
				opts.Stderr = &stderr
			}

			r, err := sandbox.Execute(context.Background(), tc.command, tc.args, opts)

			if tc.expectError {
				assert.Error(t, err)

				if tc.errorContains != "" {
					assert.ErrorContains(t, err, tc.errorContains)
				}

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedExitCode, r.ExitCode)

			if tc.expectedStdout != "" {
				assert.Equal(t, tc.expectedStdout, stdout.String())
			}

			for _, expectedContent := range tc.stdoutContains {
				assert.Contains(t, stdout.String(), expectedContent)
			}

			if tc.expectedStderr != "" {
				assert.Equal(t, tc.expectedStderr, stderr.String())
			}

			for _, expectedContent := range tc.stderrContains {
				assert.Contains(t, stderr.String(), expectedContent)
			}

			if tc.validateFn != nil {
				tc.validateFn(t, stdout.String(), stderr.String())
			}
		})
	}
}
