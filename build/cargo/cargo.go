// Package cargo contains helper functions for building Rust applications using cargo.
package cargo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

// Metadata is the cargo package metadata.
type Metadata struct {
	Name         string
	Version      string
	Dependencies []Dependency
}

// FindDependency finds the first dependency with the given name and returns it. Iff no such
// dependency can be found, it returns nil.
func (m *Metadata) FindDependency(name string) *Dependency {
	for _, d := range m.Dependencies {
		if d.Name != name {
			continue
		}

		return &d
	}
	return nil
}

// Dependency is the metadata about a dependency.
type Dependency struct {
	Name     string   `json:"name"`
	Features []string `json:"features"`
}

// HasFeature returns true iff the given feature is present among the features.
func (d *Dependency) HasFeature(feature string) bool {
	return slices.Contains(d.Features, feature)
}

// GetMetadata queries `cargo` for metadata of the package in the current working directory.
func GetMetadata() (*Metadata, error) {
	cmd := exec.Command("cargo", "metadata", "--no-deps", "--format-version", "1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize metadata process: %w", err)
	}
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start metadata process: %w", err)
	}

	dec := json.NewDecoder(stdout)
	var rawMeta struct {
		Packages []struct {
			Name         string `json:"name"`
			Version      string `json:"version"`
			Dependencies []struct {
				Name     string   `json:"name"`
				Features []string `json:"features"`
			} `json:"dependencies"`
		} `json:"packages"`
	}
	if err = dec.Decode(&rawMeta); err != nil {
		return nil, fmt.Errorf("malformed cargo metadata: %w", err)
	}
	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("metadata process failed: %w", err)
	}
	if len(rawMeta.Packages) == 0 {
		return nil, fmt.Errorf("no cargo packages found")
	}

	meta := &Metadata{
		Name:    rawMeta.Packages[0].Name,
		Version: rawMeta.Packages[0].Version,
	}
	for _, dep := range rawMeta.Packages[0].Dependencies {
		d := Dependency{
			Name:     dep.Name,
			Features: dep.Features,
		}
		meta.Dependencies = append(meta.Dependencies, d)
	}

	return meta, nil
}

// Build builds a Rust program using `cargo` in the current working directory.
func Build(release bool, target string, features []string) (string, error) {
	args := []string{"build"}
	if release {
		args = append(args, "--release")
	}
	if target != "" {
		args = append(args, "--target", target)
	}
	if features != nil {
		args = append(args, "--features", strings.Join(features, ","))
	}
	// Ensure the build process outputs JSON.
	args = append(args, "--message-format", "json")

	cmd := exec.Command("cargo", args...)
	// Parse stdout JSON messages and store stderr to buffer.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to initialize build process: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err = cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start build process: %w", err)
	}

	var executable string
	dec := json.NewDecoder(stdout)
	for {
		var output struct {
			Reason    string `json:"reason"`
			PackageID string `json:"package_id,omitempty"`
			Target    struct {
				Kind []string `json:"kind"`
			} `json:"target,omitempty"`
			Executable string `json:"executable,omitempty"`
			Message    struct {
				Rendered string `json:"rendered"`
			} `json:"message,omitempty"`
		}
		if err = dec.Decode(&output); err != nil {
			break
		}

		switch output.Reason {
		case "compiler-message":
			fmt.Println(output.Message.Rendered)
		case "compiler-artifact":
			fmt.Printf("[built] %s\n", output.PackageID)
			if len(output.Target.Kind) != 1 || output.Target.Kind[0] != "bin" {
				continue
			}

			// Extract the last built executable.
			executable = output.Executable
		default:
		}
	}
	if err = cmd.Wait(); err != nil {
		return "", fmt.Errorf("build process failed: %w\nStandard error output:\n%s", err, stderr.String())
	}

	if executable == "" {
		return "", fmt.Errorf("no executable generated")
	}
	return executable, nil
}
