package rofl

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var setAdminCmd = &cobra.Command{
	Use:     "set-admin [<app-id>] <new-admin>",
	Short:   "Change the administrator of a ROFL",
	Aliases: []string{"change-admin"},
	Args:    cobra.RangeArgs(1, 2),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		txCfg := common.GetTransactionConfig()

		var (
			rawAppID    string
			newAdminArg string
			manifest    *buildRofl.Manifest
			deployment  *buildRofl.Deployment
		)
		switch len(args) {
		case 2:
			// Direct mode: oasis rofl set-admin <app-id> <new-admin>
			rawAppID = args[0]
			newAdminArg = args[1]
		case 1:
			// Manifest mode: oasis rofl set-admin <new-admin>
			newAdminArg = args[0]
			manifest, deployment, npa = roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: true,
			})
			rawAppID = deployment.AppID
		}

		var appID rofl.AppID
		if err := appID.UnmarshalText([]byte(rawAppID)); err != nil {
			cobra.CheckErr(fmt.Errorf("malformed ROFL app ID: %w", err))
		}

		npa.MustHaveAccount()
		npa.MustHaveParaTime()

		newAdminAddr, newAdminEthAddr, err := common.ResolveLocalAccountOrAddress(npa.Network, newAdminArg)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("invalid new admin address: %w", err))
		}

		// When not in offline mode, connect to the given network endpoint.
		ctx := context.Background()
		var conn connection.Connection
		if !txCfg.Offline {
			conn, err = connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)
		}

		fmt.Printf("App ID:    %s\n", appID)

		updateBody := rofl.Update{
			ID:    appID,
			Admin: newAdminAddr,
		}

		var oldAdminAddr *types.Address
		if manifest != nil {
			// Manifest mode: use local policy, metadata, secrets.
			if deployment.Policy == nil {
				cobra.CheckErr("no policy configured in the manifest")
			}

			oldAdminAddr, _, err = common.ResolveLocalAccountOrAddress(npa.Network, deployment.Admin)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("bad current administrator address: %w", err))
			}

			updateBody.Policy = *deployment.Policy.AsDescriptor()
			updateBody.Metadata = manifest.GetMetadata(roflCommon.DeploymentName)
			updateBody.Secrets = buildRofl.PrepareSecrets(deployment.Secrets)
		} else {
			// Direct mode: reuse current policy, metadata, secrets from chain.
			if txCfg.Offline {
				cobra.CheckErr("direct mode requires network access")
			}

			appCfg, err := conn.Runtime(npa.ParaTime).ROFL.App(ctx, client.RoundLatest, appID)
			cobra.CheckErr(err)

			oldAdminAddr = appCfg.Admin
			updateBody.Policy = appCfg.Policy
			updateBody.Metadata = appCfg.Metadata
			updateBody.Secrets = appCfg.Secrets
		}

		if oldAdminAddr != nil {
			if *oldAdminAddr == *newAdminAddr {
				fmt.Println("New admin is the same as the current admin, nothing to do.")
				return
			}
			fmt.Printf("Old admin: %s\n", common.PrettyAddress(oldAdminAddr.String()))
		}

		newAdminStr := newAdminAddr.String()
		if newAdminEthAddr != nil {
			newAdminStr = newAdminEthAddr.Hex()
		}

		fmt.Printf("New admin: %s\n", common.PrettyAddress(newAdminStr))

		tx := rofl.NewUpdateTx(nil, &updateBody)

		acc := common.LoadAccount(cfg, npa.AccountName)
		sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
		cobra.CheckErr(err)

		if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
			return
		}

		// Transaction succeeded — update the manifest if available.
		if manifest != nil {
			deployment.Admin = newAdminArg
			if err = manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}
		}

		fmt.Printf("ROFL admin changed to %s.\n", common.PrettyAddress(newAdminStr))
	},
}

func init() {
	common.AddSelectorFlags(setAdminCmd)
	setAdminCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	setAdminCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
}
