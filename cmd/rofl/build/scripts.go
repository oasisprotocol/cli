package build

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
)

// runScripts executes the specified build script using the current build environment.
func runScript(manifest *buildRofl.Manifest, name string) {
	script, ok := manifest.Scripts[name]
	if !ok {
		return
	}

	fmt.Printf("Running script '%s'...\n", name)

	cmd := exec.Command( //nolint: gosec
		os.Getenv("SHELL"),
		"-c",
		script,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		cobra.CheckErr(fmt.Errorf("script '%s' failed to execute: %w", name, err))
	}
}
