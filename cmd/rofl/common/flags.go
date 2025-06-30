package common

import (
	flag "github.com/spf13/pflag"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
)

var (
	// WipeFlags is the flag for wiping the machine storage on deployment/restart.
	WipeFlags *flag.FlagSet

	// DeploymentFlags provide the deployment name.
	DeploymentFlags *flag.FlagSet

	// DeploymentName is the name of the ROFL app deployment.
	DeploymentName string

	// WipeStorage enables wiping the machine storage on deployment/restart.
	WipeStorage bool
)

func init() {
	WipeFlags = flag.NewFlagSet("", flag.ContinueOnError)
	WipeFlags.BoolVar(&WipeStorage, "wipe-storage", false, "whether to wipe machine storage")

	DeploymentFlags = flag.NewFlagSet("", flag.ContinueOnError)
	DeploymentFlags.StringVar(&DeploymentName, "deployment", buildRofl.DefaultDeploymentName, "deployment name")
}
