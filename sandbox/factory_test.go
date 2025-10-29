package sandbox

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"
)

// mockSandbox is a mock implementation of Sandbox for testing
type mockSandbox struct {
	setupCalled   bool
	executeCalled bool
	closeCalled   bool
}

func (m *mockSandbox) Setup(ctx context.Context, config SandboxSetupConfig) error {
	m.setupCalled = true
	return nil
}

func (m *mockSandbox) Execute(ctx context.Context, command string, args []string, opts SandboxExecOpts) (*SandboxExecResponse, error) {
	m.executeCalled = true
	return &SandboxExecResponse{ExitCode: 0}, nil
}

func (m *mockSandbox) WriteFile(ctx context.Context, path string, reader io.Reader) error {
	return nil
}

func (m *mockSandbox) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSandbox) Close() error {
	m.closeCalled = true
	return nil
}

func TestNewSandboxFactory(t *testing.T) {
	factory := NewSandboxFactory()
	if factory == nil {
		t.Fatal("Expected factory to be created, got nil")
	}
}

func TestCreateSandbox_Errors(t *testing.T) {
	tests := []struct {
		name          string
		sandboxType   string // empty means don't set env var
		expectedError string
	}{
		{
			name:          "missing sandbox type",
			sandboxType:   "",
			expectedError: "SANDBOX_TYPE environment variable is not set",
		},
		{
			name:          "unknown sandbox type",
			sandboxType:   "unknown_sandbox_type",
			expectedError: "unknown sandbox type: unknown_sandbox_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sandboxType != "" {
				t.Setenv(EnvSandboxType, tt.sandboxType)
			}

			factory := NewSandboxFactory()
			_, err := factory.CreateSandbox(context.Background())

			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if err.Error() != tt.expectedError {
				t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
			}
		})
	}
}

func TestRegister_CustomType(t *testing.T) {
	t.Setenv(EnvSandboxType, "custom")

	factory := NewSandboxFactory()

	customConstructor := func(ctx context.Context) (Sandbox, error) {
		return &mockSandbox{}, nil
	}

	err := factory.Register("custom", customConstructor)
	if err != nil {
		t.Fatalf("Expected successful registration, got error: %v", err)
	}

	sandbox, err := factory.CreateSandbox(context.Background())
	if err != nil {
		t.Fatalf("Expected successful sandbox creation, got error: %v", err)
	}

	if _, ok := sandbox.(*mockSandbox); !ok {
		t.Errorf("Expected mockSandbox, got %T", sandbox)
	}
}

func TestRegister_Validation(t *testing.T) {
	mockConstructor := func(ctx context.Context) (Sandbox, error) {
		return &mockSandbox{}, nil
	}

	tests := []struct {
		name          string
		sandboxType   string
		constructor   SandboxConstructor
		expectedError string
	}{
		{
			name:          "duplicate type",
			sandboxType:   SandboxTypeDocker,
			constructor:   mockConstructor,
			expectedError: "sandbox type docker is already registered",
		},
		{
			name:          "empty type",
			sandboxType:   "",
			constructor:   mockConstructor,
			expectedError: "sandbox type cannot be empty",
		},
		{
			name:          "nil constructor",
			sandboxType:   "custom",
			constructor:   nil,
			expectedError: "constructor cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewSandboxFactory()
			err := factory.Register(tt.sandboxType, tt.constructor)

			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if err.Error() != tt.expectedError {
				t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
			}
		})
	}
}

func TestParseDockerConfigFromEnv(t *testing.T) {
	tests := []struct {
		name          string
		envVars       map[string]string
		expectError   bool
		expectedError string
		validate      func(t *testing.T, config *DockerSandboxConfig)
	}{
		{
			name:          "missing required image",
			envVars:       map[string]string{},
			expectError:   true,
			expectedError: `env: required environment variable "SANDBOX_DOCKER_IMAGE" is not set`,
		},
		{
			name: "defaults applied",
			envVars: map[string]string{
				"SANDBOX_DOCKER_IMAGE": "alpine:3.18.2",
			},
			expectError: false,
			validate: func(t *testing.T, config *DockerSandboxConfig) {
				if config.Image != "alpine:3.18.2" {
					t.Errorf("Expected image 'alpine:3.18.2', got '%s'", config.Image)
				}
				if config.Socket != "/var/run/docker.sock" {
					t.Errorf("Expected socket '/var/run/docker.sock', got '%s'", config.Socket)
				}
				if config.Runtime != "runc" {
					t.Errorf("Expected runtime 'runc', got '%s'", config.Runtime)
				}
				if config.PullImageIfMissing != false {
					t.Errorf("Expected PullImageIfMissing false, got true")
				}
				if config.CreateWaitTimeout != 10*time.Second {
					t.Errorf("Expected CreateWaitTimeout 10s, got %v", config.CreateWaitTimeout)
				}
				if config.ExecWaitTimeout != 60*time.Second {
					t.Errorf("Expected ExecWaitTimeout 60s, got %v", config.ExecWaitTimeout)
				}
				if config.DestroyWaitTimeout != 10*time.Second {
					t.Errorf("Expected DestroyWaitTimeout 10s, got %v", config.DestroyWaitTimeout)
				}
			},
		},
		{
			name: "custom values",
			envVars: map[string]string{
				"SANDBOX_DOCKER_IMAGE":           "ubuntu:22.04",
				"SANDBOX_DOCKER_SOCKET":          "/custom/docker.sock",
				"SANDBOX_DOCKER_RUNTIME":         "sysbox-runc",
				"SANDBOX_DOCKER_PULL_IF_MISSING": "true",
				"SANDBOX_DOCKER_CREATE_TIMEOUT":  "20s",
				"SANDBOX_DOCKER_EXEC_TIMEOUT":    "2m",
				"SANDBOX_DOCKER_DESTROY_TIMEOUT": "5s",
				"SANDBOX_DOCKER_SKIP_WAIT":       "1",
			},
			expectError: false,
			validate: func(t *testing.T, config *DockerSandboxConfig) {
				if config.Image != "ubuntu:22.04" {
					t.Errorf("Expected image 'ubuntu:22.04', got '%s'", config.Image)
				}
				if config.Socket != "/custom/docker.sock" {
					t.Errorf("Expected socket '/custom/docker.sock', got '%s'", config.Socket)
				}
				if config.Runtime != "sysbox-runc" {
					t.Errorf("Expected runtime 'sysbox-runc', got '%s'", config.Runtime)
				}
				if !config.PullImageIfMissing {
					t.Errorf("Expected PullImageIfMissing true, got false")
				}
				if config.CreateWaitTimeout != 20*time.Second {
					t.Errorf("Expected CreateWaitTimeout 20s, got %v", config.CreateWaitTimeout)
				}
				if config.ExecWaitTimeout != 120*time.Second {
					t.Errorf("Expected ExecWaitTimeout 120s, got %v", config.ExecWaitTimeout)
				}
				if config.DestroyWaitTimeout != 5*time.Second {
					t.Errorf("Expected DestroyWaitTimeout 5s, got %v", config.DestroyWaitTimeout)
				}
				if !config.SkipWaitForRunningContainer {
					t.Errorf("Expected SkipWaitForRunningContainer true, got false")
				}
			},
		},
		{
			name: "invalid create timeout",
			envVars: map[string]string{
				"SANDBOX_DOCKER_IMAGE":          "alpine:3.18.2",
				"SANDBOX_DOCKER_CREATE_TIMEOUT": "invalid",
			},
			expectError: true,
		},
		{
			name: "invalid exec timeout",
			envVars: map[string]string{
				"SANDBOX_DOCKER_IMAGE":        "alpine:3.18.2",
				"SANDBOX_DOCKER_EXEC_TIMEOUT": "not_a_number",
			},
			expectError: true,
		},
		{
			name: "invalid destroy timeout",
			envVars: map[string]string{
				"SANDBOX_DOCKER_IMAGE":           "alpine:3.18.2",
				"SANDBOX_DOCKER_DESTROY_TIMEOUT": "abc",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			config, err := ParseDockerConfigFromEnv()

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tt.expectedError != "" && err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, &config)
				}
			}
		})
	}
}
