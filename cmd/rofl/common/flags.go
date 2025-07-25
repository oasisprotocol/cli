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

	// NoUpdateFlag is the flag for disabling the rofl.yaml manifest file update.
	NoUpdateFlag *flag.FlagSet

	// DeploymentName is the name of the ROFL app deployment.
	DeploymentName string

	// WipeStorage enables wiping the machine storage on deployment/restart.
	WipeStorage bool

	// NoUpdate disables updating the rofl.yaml manifest file.
	NoUpdate bool
)

func init() {
	WipeFlags = flag.NewFlagSet("", flag.ContinueOnError)
	WipeFlags.BoolVar(&WipeStorage, "wipe-storage", false, "whether to wipe machine storage")

	DeploymentFlags = flag.NewFlagSet("", flag.ContinueOnError)
	DeploymentFlags.StringVar(&DeploymentName, "deployment", buildRofl.DefaultDeploymentName, "deployment name")

	NoUpdateFlag = flag.NewFlagSet("", flag.ContinueOnError)
	NoUpdateFlag.BoolVar(&NoUpdate, "no-update-manifest", false, "do not update the manifest")
}
