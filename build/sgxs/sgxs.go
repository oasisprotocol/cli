// Package sgxs contains helper functions for dealing with ELF and SGXS binaries.
package sgxs

import (
	"fmt"
	"os/exec"
	"strconv"

	"github.com/oasisprotocol/cli/build/env"
	"github.com/oasisprotocol/cli/cmd/common"
)

// Elf2Sgxs converts an ELF binary built for the SGX ABI into an SGXS binary.
//
// It requires the `ftxsgx-elf2sgxs` utility to be installed.
func Elf2Sgxs(buildEnv env.ExecEnv, elfSgxPath, sgxsPath string, heapSize, stackSize, threads uint64) (err error) {
	if elfSgxPath, err = buildEnv.PathToEnv(elfSgxPath); err != nil {
		return err
	}
	if sgxsPath, err = buildEnv.PathToEnv(sgxsPath); err != nil {
		return err
	}

	args := []string{
		elfSgxPath,
		"--heap-size", strconv.FormatUint(heapSize, 10),
		"--stack-size", strconv.FormatUint(stackSize, 10),
		"--threads", strconv.FormatUint(threads, 10),
		"--output", sgxsPath,
	}

	cmd := exec.Command("ftxsgx-elf2sgxs", args...)
	if err = buildEnv.WrapCommand(cmd); err != nil {
		return err
	}
	if common.IsVerbose() {
		fmt.Println(cmd)
	}
	return cmd.Run()
}
