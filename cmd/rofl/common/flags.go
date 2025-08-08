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

	// TermFlags provide the term and count setting.
	TermFlags *flag.FlagSet

	// DeploymentName is the name of the ROFL app deployment.
	DeploymentName string

	// WipeStorage enables wiping the machine storage on deployment/restart.
	WipeStorage bool

	// NoUpdate disables updating the rofl.yaml manifest file.
	NoUpdate bool

	// Term specifies the rental base unit.
	Term string

	// TermCount specific the rental base unit multiplier.
	TermCount uint64
)

func init() {
	WipeFlags = flag.NewFlagSet("", flag.ContinueOnError)
	WipeFlags.BoolVar(&WipeStorage, "wipe-storage", false, "whether to wipe machine storage")

	DeploymentFlags = flag.NewFlagSet("", flag.ContinueOnError)
	DeploymentFlags.StringVar(&DeploymentName, "deployment", buildRofl.DefaultDeploymentName, "deployment name")

	NoUpdateFlag = flag.NewFlagSet("", flag.ContinueOnError)
	NoUpdateFlag.BoolVar(&NoUpdate, "no-update-manifest", false, "do not update the manifest")

	TermFlags = flag.NewFlagSet("", flag.ContinueOnError)
	TermFlags.StringVar(&Term, "term", "", "term to pay for in advance [hour, month, year]")
	TermFlags.Uint64Var(&TermCount, "term-count", 1, "number of terms to pay for in advance")
}
