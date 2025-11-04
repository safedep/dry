// Package sandbox defines contract for implementing sandboxed command execution tools
package sandbox

import (
	"context"
	"io"
	"time"
)

// SandboxSetupConfig is the configuration for the sandbox setup.
// This is a general config that should be supported by all sandboxes.
// Sandbox specific configs, such as docker, should be passed to the
// sandbox factory.
type SandboxSetupConfig struct {
	// WorkingDirectory is the working directory of the sandbox
	WorkingDirectory string
	// EnvironmentVariables is the environment variables to set for the sandbox
	EnvironmentVariables map[string]string
	// Labels is the labels to set for the sandbox
	Labels map[string]string
	// Entrypoint is the entrypoint to use for the sandbox
	// Behavior: When nil, the entrypoint of the base image will be used.
	// Behavior: When empty, the entrypoint of the base image will be cleared.
	// Behavior: When non-empty, the entrypoint of the base image will be overridden.
	Entrypoint *[]string
	// HealthCheckCommand is an optional health check command to execute after
	// the sandbox container is running. If provided, the command will be executed
	// repeatedly (polling every 500ms) until it succeeds (exit code 0) or the
	// setup timeout is reached. The sandbox Setup() will only succeed if the
	// health check command succeeds. Format: [command, arg1, arg2, ...]
	// Example: []string{"wget", "-O-", "localhost:8080/health"}
	// If empty or nil, no health check will be performed.
	HealthCheckCommand []string
}

// SandboxExecOpts config per execution request to be overridden from the global config
type SandboxExecOpts struct {
	// AdditionalEnv is a list of additional environment variables to set for the exec
	AdditionalEnv map[string]string

	// WorkingDirectory is the working directory to set for the exec
	WorkingDirectory string

	// SkipWaitForCompletion is whether to skip waiting for the exec to finish
	// We want to wait by default, but in some cases we may want to return
	// immediately after starting the exec.
	SkipWaitForCompletion bool

	// WaitTimeout is the timeout to wait for the exec to finish
	WaitTimeout time.Duration
}

// SandboxExecResponse is the response from the exec command
type SandboxExecResponse struct {
	// ExitCode is the exit code of the exec command
	ExitCode int
}

// Sandbox defines the contract for executing a package related
// operation within a sandboxed (or containerized) environment.
// Sandboxes are not re-usable. Create a new instance for each analysis.
type Sandbox interface {
	// Setup the sandbox. Every command executed in the sandbox will
	// run within the context of this setup. This is ideally the function
	// where sandbox implementation should allocate resources required
	// to run the commands. Also common configuration such as working
	// directory, environment variables, etc. should be set here.
	Setup(ctx context.Context, setup SandboxSetupConfig) error

	// Execute a command in the sandboxed environment.
	Execute(ctx context.Context, command string, args []string, opts SandboxExecOpts) (*SandboxExecResponse, error)

	// Write a file to the sandboxed environment
	WriteFile(ctx context.Context, path string, reader io.Reader) error

	// Read a file from the sandboxed environment
	ReadFile(ctx context.Context, path string) (io.ReadCloser, error)

	// Close the sandbox. This operation should destroy the sandbox
	// and release all resources associated with it.
	Close() error
}
