package machine

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/ed25519"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/build/rofl/scheduler"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var logsCmd = &cobra.Command{
	Use:   "logs [<machine-name> | <provider-address>:<machine-id>]",
	Short: "Show logs from the given machine",
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		_, _, machineID, _, _, providerAddr, npa := resolveMachineManifestNpa(args, &roflCommon.ManifestOptions{
			NeedAppID: true,
			NeedAdmin: false,
		})

		// Establish connection with the target network.
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		providerDsc, err := conn.Runtime(npa.ParaTime).ROFLMarket.Provider(ctx, client.RoundLatest, *providerAddr)
		cobra.CheckErr(err)

		insDsc, err := conn.Runtime(npa.ParaTime).ROFLMarket.Instance(ctx, client.RoundLatest, *providerAddr, machineID)
		cobra.CheckErr(err)

		schedulerRAKRaw, ok := insDsc.Metadata[scheduler.MetadataKeySchedulerRAK]
		if !ok {
			cobra.CheckErr("Machine is missing scheduler RAK metadata.")
		}
		var schedulerRAK ed25519.PublicKey
		if err := schedulerRAK.UnmarshalText([]byte(schedulerRAKRaw)); err != nil {
			cobra.CheckErr(fmt.Errorf("malformed scheduler RAK metadata: %w", err))
		}
		pk := types.PublicKey{PublicKey: schedulerRAK}

		schedulerDsc, err := conn.Runtime(npa.ParaTime).ROFL.AppInstance(ctx, client.RoundLatest, providerDsc.SchedulerApp, pk)
		cobra.CheckErr(err)

		client, err := scheduler.NewClient(schedulerDsc)
		cobra.CheckErr(err)

		// TODO: Cache authentication token so we don't need to re-authenticate.
		acc := common.LoadAccount(cliConfig.Global(), npa.AccountName)

		sigCtx := &signature.RichContext{
			RuntimeID:    npa.ParaTime.Namespace(),
			ChainContext: npa.Network.ChainContext,
			Base:         []byte(scheduler.StdAuthContextBase),
		}
		authRequest, err := scheduler.SignLogin(sigCtx, acc.Signer(), client.Host(), *providerAddr)
		cobra.CheckErr(err)

		err = client.Login(ctx, authRequest)
		cobra.CheckErr(err)

		logs, err := client.LogsGet(ctx, machineID, time.Time{})
		cobra.CheckErr(err)
		for _, line := range logs {
			fmt.Println(line)
		}
	},
}

func init() {
	logsCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	logsCmd.Flags().AddFlagSet(common.AnswerYesFlag)
	logsCmd.Flags().AddFlagSet(common.AccountFlag)
}
