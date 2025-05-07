package env

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExecEnv is an execution environment.
type ExecEnv interface {
	// WrapCommand modifies an existing `exec.Cmd` such that it runs in this environment.
	WrapCommand(cmd *exec.Cmd) error

	// PathFromEnv converts the given path from inside the environment into a path outside the
	// environment.
	PathFromEnv(path string) (string, error)

	// PathToEnv converts the given path from outside the environment into a path inside the
	// environment.
	PathToEnv(path string) (string, error)

	// FixPermissions ensures that the user executing this process owns the file at the given path
	// outside the environment.
	FixPermissions(path string) error

	// HasBinary returns true iff the given binary name is available in this environment.
	HasBinary(name string) bool

	// IsAvailable returns true iff the given execution environment is available.
	IsAvailable() bool
}

// NativeEnv is the native execution environment that executes all commands directly.
type NativeEnv struct{}

// NewNativeEnv creates a new native execution environment.
func NewNativeEnv() *NativeEnv {
	return &NativeEnv{}
}

// WrapCommand implements ExecEnv.
func (ne *NativeEnv) WrapCommand(*exec.Cmd) error {
	return nil
}

// PathFromEnv implements ExecEnv.
func (ne *NativeEnv) PathFromEnv(path string) (string, error) {
	return path, nil
}

// PathToEnv implements ExecEnv.
func (ne *NativeEnv) PathToEnv(path string) (string, error) {
	return path, nil
}

// FixPermissions implements ExecEnv.
func (ne *NativeEnv) FixPermissions(string) error {
	return nil
}

// HasBinary implements ExecEnv.
func (ne *NativeEnv) HasBinary(name string) bool {
	path, err := exec.LookPath(name)
	return err == nil && path != ""
}

// IsAvailable implements ExecEnv.
func (ne *NativeEnv) IsAvailable() bool {
	return true
}

// String returns a string representation of the execution environment.
func (ne *NativeEnv) String() string {
	return "native environment"
}

// DockerEnv is a Docker-based execution environment that executes all commands inside a Docker
// container using the configured image.
type DockerEnv struct {
	image   string
	volumes map[string]string
}

// NewDockerEnv creates a new Docker-based execution environment.
func NewDockerEnv(image, baseDir, dirMount string) *DockerEnv {
	return &DockerEnv{
		image: image,
		volumes: map[string]string{
			baseDir: dirMount,
		},
	}
}

// AddDirectory exposes a host directory to the container under the same path.
func (de *DockerEnv) AddDirectory(path string) {
	de.volumes[path] = path
}

// WrapCommand implements ExecEnv.
func (de *DockerEnv) WrapCommand(cmd *exec.Cmd) error {
	cmd.Err = nil // May be set by a previous exec.Command invocation.
	origArgs := cmd.Args

	var err error
	wd := cmd.Dir
	if wd == "" {
		wd, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	workDir, err := de.PathToEnv(wd)
	if err != nil {
		return fmt.Errorf("bad working directory: %w", err)
	}

	var envArgs []string //nolint: prealloc
	for _, envKV := range cmd.Env {
		envArgs = append(envArgs, "--env", envKV)
	}
	// When no environment is set, copy over any OASIS_ and ROFL_ variables.
	if len(cmd.Env) == 0 {
		for _, envKV := range os.Environ() {
			if !strings.HasPrefix(envKV, "OASIS_") && !strings.HasPrefix(envKV, "ROFL_") {
				continue
			}
			envArgs = append(envArgs, "--env", envKV)
		}
	}

	cmd.Path, err = exec.LookPath("docker")
	if err != nil {
		return fmt.Errorf("failed to find 'docker': %w", err)
	}

	cmd.Args = []string{
		"docker", "run",
		"--rm",
		"--platform", "linux/amd64",
		"--workdir", workDir,
	}
	for hostDir, bindDir := range de.volumes {
		cmd.Args = append(cmd.Args, "--volume", hostDir+":"+bindDir)
	}
	cmd.Args = append(cmd.Args, envArgs...)
	cmd.Args = append(cmd.Args, de.image)
	cmd.Args = append(cmd.Args, origArgs...)

	return nil
}

// PathFromEnv implements ExecEnv.
func (de *DockerEnv) PathFromEnv(path string) (string, error) {
	for hostDir, bindDir := range de.volumes {
		if !strings.HasPrefix(path, bindDir) {
			continue
		}
		relPath, err := filepath.Rel(bindDir, path)
		if err != nil {
			return "", fmt.Errorf("bad path: %w", err)
		}
		return filepath.Join(hostDir, relPath), nil
	}
	return "", fmt.Errorf("bad path '%s'", path)
}

// PathToEnv implements ExecEnv.
func (de *DockerEnv) PathToEnv(path string) (string, error) {
	for hostDir, bindDir := range de.volumes {
		if !strings.HasPrefix(path, hostDir) {
			continue
		}
		relPath, err := filepath.Rel(hostDir, path)
		if err != nil {
			return "", fmt.Errorf("bad path: %w", err)
		}
		return filepath.Join(bindDir, relPath), nil
	}
	return "", fmt.Errorf("bad path '%s'", path)
}

// FixPermissions implements ExecEnv.
func (de *DockerEnv) FixPermissions(path string) error {
	path, err := de.PathToEnv(path)
	if err != nil {
		return err
	}

	cmd := exec.Command("chown", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), path) //nolint: gosec
	if err = de.WrapCommand(cmd); err != nil {
		return err
	}
	return cmd.Run()
}

// HasBinary implements ExecEnv.
func (de *DockerEnv) HasBinary(string) bool {
	return true
}

// IsAvailable implements ExecEnv.
func (de *DockerEnv) IsAvailable() bool {
	path, err := exec.LookPath("docker")
	return err == nil && path != ""
}

// String returns a string representation of the execution environment.
func (de *DockerEnv) String() string {
	return "Docker"
}
