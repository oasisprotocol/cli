package build

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/build/env"
	buildRofl "github.com/oasisprotocol/cli/build/rofl"
)

// runScripts executes the specified build script using the current build environment.
func runScript(manifest *buildRofl.Manifest, name string, buildEnv env.ExecEnv, useContainer bool) {
	script, ok := manifest.Scripts[name]
	if !ok {
		return
	}

	fmt.Printf("Running script '%s'...\n", name)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell, "-c", script) //nolint: gosec

	if useContainer {
		if err := buildEnv.WrapCommand(cmd); err != nil {
			cobra.CheckErr(fmt.Errorf("script '%s' failed to wrap for container: %w", name, err))
		}
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		cobra.CheckErr(fmt.Errorf("script '%s' failed to execute: %w", name, err))
	}
}
