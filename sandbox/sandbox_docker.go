package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/safedep/dry/log"
)

type DockerSandboxConfig struct {
	// Underline runtime (default is runc)
	Runtime string

	// The image to use to create the container.
	Image string

	// Pull the image if it is not already present on the container daemon.
	PullImageIfMissing bool

	// The path to the docker socket.
	// If not provided, the default unix socket will be used.
	Socket string

	// Timeout to wait for the container to be running
	CreateWaitTimeout time.Duration

	// Timeout to wait for the exec to finish
	// This is the global timeout for all execs, can be overridden per execution
	ExecWaitTimeout time.Duration

	// Timeout to wait for the container to be destroyed
	DestroyWaitTimeout time.Duration

	// Process to create that will block when creating container
	InitCommand []string

	// Skip waiting for container to be running
	SkipWaitForRunningContainer bool
}

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

	resp, err := s.client.ContainerCreate(ctx, &container.Config{
		Image:        s.config.Image,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   s.setup.WorkingDirectory,
		Env:          environmentVariables,
		Cmd:          s.config.InitCommand,
		Labels:       s.setup.Labels,
	}, &container.HostConfig{
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

	statusCh, errCh := s.client.ContainerWait(timeoutContext, s.containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("failed to wait for container: %w", err)
		}
	case <-statusCh:
	}

	return nil
}

func (s *dockerSandbox) Execute(ctx context.Context, command string, args []string, opts SandboxExecOpts) (*SandboxExecResponse, error) {
	if s.containerID == "" {
		return nil, fmt.Errorf("container not created")
	}

	log.Debugf("Executing command: %s %v", command, args)

	commandWithArgs := append([]string{command}, args...)
	execConfig := container.ExecOptions{
		Cmd:          commandWithArgs,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   s.setup.WorkingDirectory,
		Tty:          true,
		Privileged:   false,
	}

	if opts.WorkingDirectory != "" {
		execConfig.WorkingDir = opts.WorkingDirectory
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

	err = s.client.ContainerExecStart(ctx, execSession.ID, container.ExecStartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start exec: %w", err)
	}

	if opts.SkipWaitForCompletion {
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
		case <-time.After(500 * time.Millisecond):
			// Continue waiting
		}
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
		if err != nil {
			return nil, fmt.Errorf("failed to read tar archive: %w", err)
		}

		baseName := filepath.Base(path)
		if header.Name == baseName {
			return io.NopCloser(tarReader), nil
		}
	}
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

	s.containerID = ""
	log.Debugf("Stopped and removed container: %s", s.containerID)

	return nil
}
