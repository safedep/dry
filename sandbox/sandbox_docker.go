package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/safedep/dry/log"
	"github.com/safedep/dry/utils"
)

const defaultDockerOperationWaitTime = time.Millisecond * 500

type DockerSandboxConfig struct {
	// Docker host runtime (default is runc)
	Runtime string `env:"SANDBOX_DOCKER_RUNTIME" envDefault:"runc"`

	// The image to use to create the container.
	Image string `env:"SANDBOX_DOCKER_IMAGE,required"`

	// Pull the image if it is not already present on the container daemon.
	PullImageIfMissing bool `env:"SANDBOX_DOCKER_PULL_IF_MISSING" envDefault:"false"`

	// The path to the docker socket.
	// If not provided, the default unix socket will be used.
	Socket string `env:"SANDBOX_DOCKER_SOCKET" envDefault:"/var/run/docker.sock"`

	// Timeout to wait for the container to be running
	CreateWaitTimeout time.Duration `env:"SANDBOX_DOCKER_CREATE_TIMEOUT" envDefault:"10s"`

	// Timeout to wait for the exec to finish
	// This is the global timeout for all execs, can be overridden per execution
	ExecWaitTimeout time.Duration `env:"SANDBOX_DOCKER_EXEC_TIMEOUT" envDefault:"60s"`

	// Timeout to wait for the container to be destroyed
	DestroyWaitTimeout time.Duration `env:"SANDBOX_DOCKER_DESTROY_TIMEOUT" envDefault:"10s"`

	// Process to create that will block when creating container
	InitCommand []string `env:"SANDBOX_DOCKER_INIT_COMMAND" envSeparator:"," envDefault:"/bin/sh,-c,sleep 100d"`

	// Skip waiting for container to be running
	SkipWaitForRunningContainer bool `env:"SANDBOX_DOCKER_SKIP_WAIT" envDefault:"false"`
}

// DefaultDockerSandboxConfig creates default config for the docker sandbox
func DefaultDockerSandboxConfig(image string) DockerSandboxConfig {
	return DockerSandboxConfig{
		Runtime:            "runc",
		Image:              image,
		Socket:             "/var/run/docker.sock",
		CreateWaitTimeout:  10 * time.Second,
		ExecWaitTimeout:    60 * time.Second,
		DestroyWaitTimeout: 10 * time.Second,
		InitCommand:        []string{"/bin/sh", "-c", "sleep 100d"},
	}
}

// ParseDockerConfigFromEnv creates a DockerSandboxConfig from environment variables.
// Environment variables are parsed using the caarlos0/env package.
// See DockerSandboxConfig struct tags for the full list of supported environment variables.
func ParseDockerConfigFromEnv() (DockerSandboxConfig, error) {
	return utils.ParseEnvToStruct[DockerSandboxConfig]()
}

type dockerSandbox struct {
	config      DockerSandboxConfig
	client      *client.Client
	setup       SandboxSetupConfig
	containerID string
}

var _ Sandbox = &dockerSandbox{}

// NewDockerSandbox creates a new docker sandbox.
// This sandbox is NOT re-usable.
func NewDockerSandbox(config DockerSandboxConfig) (*dockerSandbox, error) {
	client, err := client.NewClientWithOpts(
		client.WithAPIVersionNegotiation(),
		client.WithHost(fmt.Sprintf("unix://%s", config.Socket)),
		// Allow the client to be configured via environment variables.
		client.FromEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &dockerSandbox{config: config, client: client}, nil
}

func (s *dockerSandbox) Setup(ctx context.Context, config SandboxSetupConfig) error {
	if s.containerID != "" {
		return fmt.Errorf("container already created")
	}

	log.Debugf("Setting up docker sandbox with image: %s", s.config.Image)

	if s.config.PullImageIfMissing {
		images, err := s.client.ImageList(ctx, image.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list images: %w", err)
		}

		imageExists := false
		for _, image := range images {
			if slices.Contains(image.RepoTags, s.config.Image) {
				imageExists = true
				break
			}
		}

		if !imageExists {
			log.Debugf("Pulling image: %s", s.config.Image)

			r, err := s.client.ImagePull(ctx, s.config.Image, image.PullOptions{})
			if err != nil {
				return fmt.Errorf("failed to pull image: %w", err)
			}

			defer r.Close()

			// Wait for the image to be pulled
			_, err = io.ReadAll(r)
			if err != nil {
				return fmt.Errorf("failed to read image pull response: %w", err)
			}
		}
	}

	s.setup = config

	// Set sane defaults if not provided
	if s.setup.WorkingDirectory == "" {
		s.setup.WorkingDirectory = "/"
	}

	log.Debugf("Creating container with image: %s", s.config.Image)

	environmentVariables := []string{}
	for key, value := range s.setup.EnvironmentVariables {
		environmentVariables = append(environmentVariables, fmt.Sprintf("%s=%s", key, value))
	}

	containerConfig := &container.Config{
		Image:        s.config.Image,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   s.setup.WorkingDirectory,
		Env:          environmentVariables,
		Cmd:          s.config.InitCommand,
		Labels:       s.setup.Labels,
	}

	if s.setup.Entrypoint != nil {
		containerConfig.Entrypoint = *s.setup.Entrypoint
	}

	resp, err := s.client.ContainerCreate(ctx, containerConfig, &container.HostConfig{
		Runtime: s.config.Runtime,
	}, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	log.Debugf("Created container with ID: %s", resp.ID)
	s.containerID = resp.ID

	// Start the container
	err = s.client.ContainerStart(ctx, s.containerID, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	log.Debugf("Started container with ID: %s", s.containerID)

	if s.config.SkipWaitForRunningContainer {
		return nil
	}

	log.Debugf("Waiting for container to be in running state")

	// Wait for the container to be running
	timeoutContext, timeoutCancel := context.WithTimeout(ctx, s.config.CreateWaitTimeout)
	defer timeoutCancel()

	waitTicker := time.NewTicker(defaultDockerOperationWaitTime)
	defer waitTicker.Stop()

	for {
		select {
		case <-timeoutContext.Done():
			return fmt.Errorf("container failed to start within %s", s.config.CreateWaitTimeout)
		case <-waitTicker.C:
			log.Debugf("Checking if container is running")

			container, err := s.client.ContainerInspect(timeoutContext, s.containerID)
			if err != nil {
				return fmt.Errorf("failed to inspect container: %w", err)
			}

			if container.State.Dead {
				return fmt.Errorf("container failed to start")
			}

			if container.State.Running {
				if len(s.setup.HealthCheckCommand) > 0 {
					log.Debugf("Container is running, executing health check command: %v", s.setup.HealthCheckCommand)

					err := s.executeHealthCheck(timeoutContext)
					if err != nil {
						return fmt.Errorf("health check failed: %w", err)
					}

					log.Debugf("Health check succeeded")
				}

				return nil
			}
		}
	}
}

// executeHealthCheck runs the health check command repeatedly until it succeeds or times out
func (s *dockerSandbox) executeHealthCheck(ctx context.Context) error {
	if len(s.setup.HealthCheckCommand) == 0 {
		return nil
	}

	command := s.setup.HealthCheckCommand[0]
	args := []string{}

	if len(s.setup.HealthCheckCommand) > 1 {
		args = s.setup.HealthCheckCommand[1:]
	}

	healthCheckTicker := time.NewTicker(defaultDockerOperationWaitTime)
	defer healthCheckTicker.Stop()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("health check command %v timed out (last error: %w)", s.setup.HealthCheckCommand, lastErr)
			}

			return fmt.Errorf("health check command %v timed out", s.setup.HealthCheckCommand)
		case <-healthCheckTicker.C:
			log.Debugf("Executing health check: %s %v", command, args)

			resp, err := s.Execute(ctx, command, args, SandboxExecOpts{
				WaitTimeout: defaultDockerOperationWaitTime,
			})
			if err != nil {
				log.Debugf("Health check execution error: %v", err)
				lastErr = err
				continue
			}

			if resp.ExitCode == 0 {
				log.Debugf("Health check succeeded with exit code 0")
				return nil
			}

			lastErr = fmt.Errorf("exit code %d", resp.ExitCode)
			log.Debugf("Health check failed with exit code %d, retrying...", resp.ExitCode)
		}
	}
}

func (s *dockerSandbox) Execute(ctx context.Context, command string, args []string, opts SandboxExecOpts) (*SandboxExecResponse, error) {
	if s.containerID == "" {
		return nil, fmt.Errorf("container not created")
	}

	log.Debugf("Executing command: %s %v", command, args)

	// Override working dir if set in exec opts
	workingDir := s.setup.WorkingDirectory
	if opts.WorkingDirectory != "" {
		workingDir = opts.WorkingDirectory
	}

	// Determine if we need to attach IO streams
	attachStdin := opts.Stdin != nil
	attachStdout := opts.Stdout != nil
	attachStderr := opts.Stderr != nil
	needsAttach := attachStdin || attachStdout || attachStderr

	// Fail fast when both attachment and skip completion is requested
	if needsAttach && opts.SkipWaitForCompletion {
		return nil, fmt.Errorf("cannot skip completion when input/output streams are provided")
	}

	// Disable TTY when capturing any output to avoid \r\n line endings
	// and to properly separate stdout/stderr when both are captured
	enableTty := !attachStdout && !attachStderr

	commandWithArgs := append([]string{command}, args...)
	execConfig := container.ExecOptions{
		Cmd:          commandWithArgs,
		AttachStdin:  attachStdin,
		AttachStdout: attachStdout,
		AttachStderr: attachStderr,
		WorkingDir:   workingDir,
		Tty:          enableTty,
		Privileged:   false,
	}

	if len(opts.AdditionalEnv) > 0 {
		additionalEnv := []string{}
		for key, value := range opts.AdditionalEnv {
			additionalEnv = append(additionalEnv, fmt.Sprintf("%s=%s", key, value))
		}

		execConfig.Env = additionalEnv
	}

	execSession, err := s.client.ContainerExecCreate(ctx, s.containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	if needsAttach {
		resp, err := s.client.ContainerExecAttach(ctx, execSession.ID, container.ExecStartOptions{
			Tty: enableTty,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to attach to exec: %w", err)
		}

		defer resp.Close()

		var ioErr error
		ioDone := make(chan struct{})

		go func() {
			defer close(ioDone)
			ioErr = s.handleExecIO(ctx, &resp, opts, enableTty)
		}()

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("exec cancelled: %w", ctx.Err())
		case <-ioDone:
			if ioErr != nil {
				log.Debugf("IO error during exec: %v", ioErr)
			}
		}
	} else {
		err = s.client.ContainerExecStart(ctx, execSession.ID, container.ExecStartOptions{
			Tty: enableTty,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to start exec: %w", err)
		}
	}

	// We will never skip completion if any of the streams are being attached
	// to avoid resource leak while dealing with stream data
	if !needsAttach && opts.SkipWaitForCompletion {
		return &SandboxExecResponse{}, nil
	}

	log.Debugf("Waiting for exec to finish")

	timeout := s.config.ExecWaitTimeout
	if opts.WaitTimeout != 0 {
		timeout = opts.WaitTimeout
	}

	execWaitContext, execWaitCancel := context.WithTimeout(ctx, timeout)
	defer execWaitCancel()

	for {
		execInfo, err := s.client.ContainerExecInspect(execWaitContext, execSession.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect exec: %w", err)
		}

		if !execInfo.Running {
			return &SandboxExecResponse{
				ExitCode: execInfo.ExitCode,
			}, nil
		}

		select {
		case <-execWaitContext.Done():
			return nil, fmt.Errorf("exec timed out")
		case <-time.After(defaultDockerOperationWaitTime):
			// Continue waiting
		}
	}
}

// handleExecIO handles copying data between the exec streams and the provided readers/writers
func (s *dockerSandbox) handleExecIO(ctx context.Context, resp *types.HijackedResponse, opts SandboxExecOpts, tty bool) error {
	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	// Handle stdin
	if opts.Stdin != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := io.Copy(resp.Conn, opts.Stdin)
			// Always close write side after copying, even if there was an error
			resp.CloseWrite()
			if err != nil && err != io.EOF {
				errChan <- fmt.Errorf("stdin copy error: %w", err)
			}
		}()
	} else {
		// Close write side if no stdin
		resp.CloseWrite()
	}

	// Handle stdout and stderr
	if tty {
		// In TTY mode, stdout and stderr are merged
		if opts.Stdout != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := io.Copy(opts.Stdout, resp.Reader)
				if err != nil && err != io.EOF {
					errChan <- fmt.Errorf("stdout copy error: %w", err)
				}
			}()
		}
	} else {
		// Non-TTY mode: separate stdout and stderr
		if opts.Stdout != nil || opts.Stderr != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Use stdcopy to demultiplex Docker's stream format
				// Pass nil for streams we don't want to capture
				stdout := opts.Stdout
				stderr := opts.Stderr
				if stdout == nil {
					stdout = io.Discard
				}
				if stderr == nil {
					stderr = io.Discard
				}
				_, err := stdcopy.StdCopy(stdout, stderr, resp.Reader)
				if err != nil && err != io.EOF {
					errChan <- fmt.Errorf("stdout/stderr copy error: %w", err)
				}
			}()
		}
	}

	// Wait for all IO operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		// Return first error, but continue waiting for other goroutines
		return err
	case <-done:
		return nil
	}
}

func (s *dockerSandbox) WriteFile(ctx context.Context, path string, reader io.Reader) error {
	if s.containerID == "" {
		return fmt.Errorf("container not created")
	}

	log.Debugf("Writing file to container: %s", path)

	tarBuffer := new(bytes.Buffer)
	tarWriter := tar.NewWriter(tarBuffer)

	content, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	header := &tar.Header{
		Name: filepath.Base(path),
		Mode: 0o644,
		Size: int64(len(content)),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	if _, err := tarWriter.Write(content); err != nil {
		return fmt.Errorf("failed to write file content to tar: %w", err)
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	dirPath := filepath.Dir(path)
	if dirPath == "" {
		dirPath = "/"
	}

	if err := s.client.CopyToContainer(ctx, s.containerID, dirPath, bytes.NewReader(tarBuffer.Bytes()),
		container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("failed to copy file to container: %w", err)
	}

	return nil
}

func (s *dockerSandbox) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if s.containerID == "" {
		return nil, fmt.Errorf("container not created")
	}

	log.Debugf("Reading file from container: %s", path)

	reader, _, err := s.client.CopyFromContainer(ctx, s.containerID, path)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file from container: %w", err)
	}

	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar archive: %w", err)
		}

		baseName := filepath.Base(path)
		if header.Name == baseName {
			return io.NopCloser(tarReader), nil
		}
	}

	return nil, fmt.Errorf("could not find the file %s in container %s", path, s.containerID)
}

func (s *dockerSandbox) Close() error {
	if s.containerID == "" {
		return nil
	}

	log.Debugf("Stopping container: %s", s.containerID)

	timeoutInSeconds := int(s.config.DestroyWaitTimeout.Seconds())
	err := s.client.ContainerStop(context.Background(), s.containerID, container.StopOptions{
		Signal:  "SIGKILL",
		Timeout: &timeoutInSeconds,
	})
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	err = s.client.ContainerRemove(context.Background(), s.containerID, container.RemoveOptions{
		RemoveVolumes: true,
		RemoveLinks:   false,
		Force:         true,
	})
	if err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	log.Debugf("Stopped and removed container: %s", s.containerID)
	s.containerID = ""

	return nil
}
