package rofl

import (
	"fmt"

	"github.com/spf13/cobra"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
)

var setDefaultCmd = &cobra.Command{
	Use:   "set-default <deployment>",
	Short: "Sets the given deployment as the default deployment",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		name := args[0]

		manifest, err := buildRofl.LoadManifest()
		cobra.CheckErr(err)

		err = manifest.SetDefaultDeployment(name)
		cobra.CheckErr(err)

		err = manifest.Save()
		cobra.CheckErr(err)

		fmt.Printf("Set '%s' as the default deployment.\n", name)
	},
}
