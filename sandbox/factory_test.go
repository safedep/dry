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

func TestCreateSandbox_MissingSandboxType(t *testing.T) {
	factory := NewSandboxFactory()
	_, err := factory.CreateSandbox(context.Background())

	if err == nil {
		t.Fatal("Expected error when SANDBOX_TYPE is not set, got nil")
	}

	expectedError := "SANDBOX_TYPE environment variable is not set"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestCreateSandbox_UnknownType(t *testing.T) {
	t.Setenv(EnvSandboxType, "unknown_sandbox_type")

	factory := NewSandboxFactory()
	_, err := factory.CreateSandbox(context.Background())

	if err == nil {
		t.Fatal("Expected error for unknown sandbox type, got nil")
	}

	expectedError := "unknown sandbox type: unknown_sandbox_type"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
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

func TestRegister_DuplicateType(t *testing.T) {
	factory := NewSandboxFactory()

	customConstructor := func(ctx context.Context) (Sandbox, error) {
		return &mockSandbox{}, nil
	}

	// Try to register docker type again (it's already registered)
	err := factory.Register(SandboxTypeDocker, customConstructor)
	if err == nil {
		t.Fatal("Expected error when registering duplicate type, got nil")
	}

	expectedError := "sandbox type docker is already registered"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestRegister_EmptyType(t *testing.T) {
	factory := NewSandboxFactory()

	customConstructor := func(ctx context.Context) (Sandbox, error) {
		return &mockSandbox{}, nil
	}

	err := factory.Register("", customConstructor)
	if err == nil {
		t.Fatal("Expected error when registering empty type, got nil")
	}

	expectedError := "sandbox type cannot be empty"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestRegister_NilConstructor(t *testing.T) {
	factory := NewSandboxFactory()

	err := factory.Register("custom", nil)
	if err == nil {
		t.Fatal("Expected error when registering nil constructor, got nil")
	}

	expectedError := "constructor cannot be nil"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestParseDockerConfigFromEnv_MissingImage(t *testing.T) {
	_, err := ParseDockerConfigFromEnv()
	if err == nil {
		t.Fatal("Expected error when SANDBOX_DOCKER_IMAGE is not set, got nil")
	}

	// caarlos0/env returns this error message for required fields
	expectedError := `env: required environment variable "SANDBOX_DOCKER_IMAGE" is not set`
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestParseDockerConfigFromEnv_WithDefaults(t *testing.T) {
	t.Setenv("SANDBOX_DOCKER_IMAGE", "alpine:3.18.2")

	config, err := ParseDockerConfigFromEnv()
	if err != nil {
		t.Fatalf("Expected successful parsing, got error: %v", err)
	}

	// Check defaults
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
}

func TestParseDockerConfigFromEnv_WithCustomValues(t *testing.T) {
	t.Setenv("SANDBOX_DOCKER_IMAGE", "ubuntu:22.04")
	t.Setenv("SANDBOX_DOCKER_SOCKET", "/custom/docker.sock")
	t.Setenv("SANDBOX_DOCKER_RUNTIME", "sysbox-runc")
	t.Setenv("SANDBOX_DOCKER_PULL_IF_MISSING", "true")
	t.Setenv("SANDBOX_DOCKER_CREATE_TIMEOUT", "20s")
	t.Setenv("SANDBOX_DOCKER_EXEC_TIMEOUT", "2m")
	t.Setenv("SANDBOX_DOCKER_DESTROY_TIMEOUT", "5s")
	t.Setenv("SANDBOX_DOCKER_SKIP_WAIT", "1")

	config, err := ParseDockerConfigFromEnv()
	if err != nil {
		t.Fatalf("Expected successful parsing, got error: %v", err)
	}

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
}

func TestParseDockerConfigFromEnv_InvalidTimeout(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		value  string
	}{
		{"invalid create timeout", "SANDBOX_DOCKER_CREATE_TIMEOUT", "invalid"},
		{"invalid exec timeout", "SANDBOX_DOCKER_EXEC_TIMEOUT", "not_a_number"},
		{"invalid destroy timeout", "SANDBOX_DOCKER_DESTROY_TIMEOUT", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SANDBOX_DOCKER_IMAGE", "alpine:3.18.2")
			t.Setenv(tt.envVar, tt.value)

			_, err := ParseDockerConfigFromEnv()
			if err == nil {
				t.Fatalf("Expected error for invalid %s, got nil", tt.envVar)
			}
		})
	}
}
