package rofl

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var setAdminCmd = &cobra.Command{
	Use:     "set-admin <new-admin>",
	Short:   "Change the administrator of the application in ROFL",
	Aliases: []string{"change-admin"},
	Args:    cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		txCfg := common.GetTransactionConfig()

		manifest, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
			NeedAppID: true,
			NeedAdmin: true,
		})

		var appID rofl.AppID
		if err := appID.UnmarshalText([]byte(deployment.AppID)); err != nil {
			cobra.CheckErr(fmt.Errorf("malformed ROFL app ID: %w", err))
		}

		npa.MustHaveAccount()
		npa.MustHaveParaTime()

		if deployment.Policy == nil {
			cobra.CheckErr("no policy configured in the manifest")
		}

		oldAdminAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, deployment.Admin)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("bad current administrator address: %w", err))
		}

		newAdminAddr, newAdminEthAddr, err := common.ResolveLocalAccountOrAddress(npa.Network, args[0])
		if err != nil {
			cobra.CheckErr(fmt.Errorf("invalid new admin address: %w", err))
		}

		if *oldAdminAddr == *newAdminAddr {
			fmt.Println("New admin is the same as the current admin, nothing to do.")
			return
		}

		// When not in offline mode, connect to the given network endpoint.
		ctx := context.Background()
		var conn connection.Connection
		if !txCfg.Offline {
			conn, err = connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)
		}

		newAdminStr := newAdminAddr.String()
		if newAdminEthAddr != nil {
			newAdminStr = newAdminEthAddr.Hex()
		}

		fmt.Printf("App ID:    %s\n", deployment.AppID)
		fmt.Printf("Old admin: %s\n", common.PrettyAddress(oldAdminAddr.String()))
		fmt.Printf("New admin: %s\n", common.PrettyAddress(newAdminStr))

		secrets := buildRofl.PrepareSecrets(deployment.Secrets)

		tx := rofl.NewUpdateTx(nil, &rofl.Update{
			ID:       appID,
			Policy:   *deployment.Policy.AsDescriptor(),
			Admin:    newAdminAddr,
			Metadata: manifest.GetMetadata(roflCommon.DeploymentName),
			Secrets:  secrets,
		})

		acc := common.LoadAccount(cliConfig.Global(), npa.AccountName)
		sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
		cobra.CheckErr(err)

		if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
			return
		}

		// Transaction succeeded — update the manifest with the new admin.
		deployment.Admin = args[0]
		if err = manifest.Save(); err != nil {
			cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
		}

		fmt.Printf("ROFL admin changed to %s.\n", common.PrettyAddress(newAdminStr))
	},
}

func init() {
	common.AddAccountFlag(setAdminCmd)
	setAdminCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	setAdminCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
}
