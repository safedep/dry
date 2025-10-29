package sandbox

import (
	"context"
	"fmt"
	"os"
	"sync"
)

const (
	// EnvSandboxType is the environment variable that specifies the sandbox type
	EnvSandboxType = "SANDBOX_TYPE"

	// SandboxTypeDocker represents the Docker sandbox type
	SandboxTypeDocker = "docker"
)

// SandboxConstructor is a function that creates a sandbox from environment variables
type SandboxConstructor func(ctx context.Context) (Sandbox, error)

// SandboxFactory creates sandboxes based on environment configuration
type SandboxFactory interface {
	// CreateSandbox creates a new sandbox instance based on environment variables.
	// The sandbox type is determined by the SANDBOX_TYPE environment variable.
	// Returns an error if SANDBOX_TYPE is not set or if the sandbox type is unknown.
	CreateSandbox(ctx context.Context) (Sandbox, error)

	// Register registers a custom sandbox type with a constructor function.
	// This allows extending the factory with new sandbox types at runtime.
	Register(sandboxType string, constructor SandboxConstructor) error
}

// defaultSandboxFactory is the default implementation of SandboxFactory
type defaultSandboxFactory struct {
	mu           sync.RWMutex
	constructors map[string]SandboxConstructor
}

// NewSandboxFactory creates a new SandboxFactory with built-in sandbox types registered
func NewSandboxFactory() SandboxFactory {
	factory := &defaultSandboxFactory{
		constructors: make(map[string]SandboxConstructor),
	}

	// Register built-in sandbox types
	_ = factory.Register(SandboxTypeDocker, newDockerSandboxFromEnv)

	return factory
}

// CreateSandbox creates a new sandbox based on environment configuration
func (f *defaultSandboxFactory) CreateSandbox(ctx context.Context) (Sandbox, error) {
	sandboxType := os.Getenv(EnvSandboxType)
	if sandboxType == "" {
		return nil, fmt.Errorf("SANDBOX_TYPE environment variable is not set")
	}

	f.mu.RLock()
	constructor, exists := f.constructors[sandboxType]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown sandbox type: %s", sandboxType)
	}

	return constructor(ctx)
}

// Register registers a custom sandbox type with a constructor function
func (f *defaultSandboxFactory) Register(sandboxType string, constructor SandboxConstructor) error {
	if sandboxType == "" {
		return fmt.Errorf("sandbox type cannot be empty")
	}

	if constructor == nil {
		return fmt.Errorf("constructor cannot be nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.constructors[sandboxType]; exists {
		return fmt.Errorf("sandbox type %s is already registered", sandboxType)
	}

	f.constructors[sandboxType] = constructor
	return nil
}

// newDockerSandboxFromEnv creates a Docker sandbox from environment variables
func newDockerSandboxFromEnv(ctx context.Context) (Sandbox, error) {
	config, err := ParseDockerConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to parse docker config from environment: %w", err)
	}

	return NewDockerSandbox(config)
}
