package cmd

import (
	"context"
	"fmt"

	selfupdate "github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/version"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Download and install the newest Oasis CLI release",
	Long: `Checks GitHub releases for a newer Oasis CLI version that matches
your OS/ARCH, downloads it and atomically replaces the current binary.`,
	Args: cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		repo := selfupdate.ParseSlug("oasisprotocol/cli")
		current := version.Software

		rel, found, err := selfupdate.DetectLatest(context.Background(), repo)
		cobra.CheckErr(err)

		if !found || !rel.GreaterThan(current) {
			fmt.Printf("Oasis CLI is already up‑to‑date (%s).\n", current)
			return
		}

		exe, err := selfupdate.ExecutablePath()
		cobra.CheckErr(err)

		err = selfupdate.UpdateTo(context.Background(), rel.AssetURL, rel.AssetName, exe)
		cobra.CheckErr(err)

		fmt.Printf("✅  Updated to %s\n\nRelease notes:\n%s\n",
			rel.Version(), rel.ReleaseNotes)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
