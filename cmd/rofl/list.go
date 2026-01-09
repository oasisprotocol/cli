package rofl

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/table"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List ROFL applications",
	Long: `List all ROFL applications registered on the selected paratime.

This command queries on-chain ROFL application data and displays application IDs,
names, and admin addresses.

Use --format json for machine-readable output.`,
	Args: cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		npa.MustHaveParaTime()

		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		// Query all ROFL apps from the chain.
		apps, err := conn.Runtime(npa.ParaTime).ROFL.Apps(ctx, client.RoundLatest)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("failed to query ROFL apps: %w", err))
		}

		// Sort apps by ID for consistent output.
		sort.Slice(apps, func(i, j int) bool {
			return apps[i].ID.String() < apps[j].ID.String()
		})

		// Output format handling.
		if common.OutputFormat() == common.FormatJSON {
			outputAppsJSON(apps)
			return
		}

		if len(apps) == 0 {
			fmt.Println("No ROFL applications found.")
			return
		}

		outputAppsText(apps)
	},
}

// outputAppsJSON returns apps in JSON format.
func outputAppsJSON(apps []*rofl.AppConfig) {
	output := make([]common.JSONPrettyPrintRoflAppConfig, 0, len(apps))
	for _, app := range apps {
		output = append(output, common.JSONPrettyPrintRoflAppConfig(*app))
	}

	data, err := common.PrettyJSONMarshal(output)
	cobra.CheckErr(err)
	fmt.Println(string(data))
}

// outputAppsText returns apps in human-readable table format.
func outputAppsText(apps []*rofl.AppConfig) {
	tbl := table.New()
	tbl.Header("App ID", "Name", "Version", "Admin")

	rows := make([][]string, 0, len(apps))
	for _, app := range apps {
		// Extract name from metadata.
		name := app.Metadata["net.oasis.rofl.name"]

		// Extract version from metadata.
		version := app.Metadata["net.oasis.rofl.version"]

		// Format admin address.
		var admin string
		if app.Admin != nil {
			admin = app.Admin.String()
		}

		rows = append(rows, []string{
			app.ID.String(),
			name,
			version,
			admin,
		})
	}

	cobra.CheckErr(tbl.Bulk(rows))
	cobra.CheckErr(tbl.Render())
}

func init() {
	common.AddSelectorNPFlags(listCmd)
	listCmd.Flags().AddFlagSet(common.FormatFlag)
}
