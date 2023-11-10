package account

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/entity"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	entityOutputFile string

	entityCmd = &cobra.Command{
		Use:   "entity",
		Short: "Entity management in the network's registry",
	}

	entityInitCmd = &cobra.Command{
		Use:   "init",
		Short: "Init an empty entity file",
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			if npa.Account == nil {
				cobra.CheckErr("no accounts configured in your wallet")
			}

			// Load the account and ensure it corresponds to the entity.
			acc := common.LoadAccount(cfg, npa.AccountName)
			signer := acc.ConsensusSigner()
			if signer == nil {
				cobra.CheckErr(fmt.Errorf("account '%s' does not support signing consensus transactions", npa.AccountName))
			}

			descriptor := entity.Entity{
				Versioned: cbor.NewVersioned(entity.LatestDescriptorVersion),
				ID:        signer.Public(),
				Nodes:     []signature.PublicKey{},
			}

			// Marshal to JSON, add explicit "nodes" field to hint user.
			descriptorStr, err := json.Marshal(descriptor)
			cobra.CheckErr(err)
			var rawJSON map[string]interface{}
			err = json.Unmarshal(descriptorStr, &rawJSON)
			cobra.CheckErr(err)
			rawJSON["nodes"] = []string{}
			descriptorStr, err = common.PrettyJSONMarshal(rawJSON)
			cobra.CheckErr(err)

			if entityOutputFile == "" {
				fmt.Printf("%s\n", descriptorStr)
			} else {
				err = os.WriteFile(entityOutputFile, descriptorStr, 0o644) //nolint: gosec
				cobra.CheckErr(err)
			}
		},
	}

	entityRegisterCmd = &cobra.Command{
		Use:   "register <entity.json>",
		Short: "Register or update account entity in registry",
		Long:  "Register your account and nodes as entity in the network registry or update the existing entry.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			filename := args[0]

			if npa.Account == nil {
				cobra.CheckErr("no accounts configured in your wallet")
			}

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			// Load entity descriptor.
			rawDescriptor, err := os.ReadFile(filename)
			cobra.CheckErr(err)

			// Parse entity descriptor.
			var descriptor entity.Entity
			if err = json.Unmarshal(rawDescriptor, &descriptor); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed entity descriptor: %w", err))
			}

			// Load the account and ensure it corresponds to the entity.
			acc := common.LoadAccount(cfg, npa.AccountName)
			signer := acc.ConsensusSigner()
			if signer == nil {
				cobra.CheckErr(fmt.Errorf("account '%s' does not support signing consensus transactions", npa.AccountName))
			}
			if !signer.Public().Equal(descriptor.ID) {
				cobra.CheckErr(fmt.Errorf("entity ID '%s' does not correspond to selected account '%s' (%s)",
					descriptor.ID, npa.AccountName, signer.Public()))
			}

			// Sign entity descriptor.
			fmt.Println("Signing the entity descriptor...")
			fmt.Println("(In case you are using a hardware-based signer you may need to confirm on device.)")
			sigDescriptor, err := entity.SignEntity(signer, registry.RegisterEntitySignatureContext, &descriptor)
			cobra.CheckErr(err)

			// Prepare transaction.
			tx := registry.NewRegisterEntityTx(0, nil, sigDescriptor)

			sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa.ParaTime, conn, sigTx, nil, nil)
		},
	}

	entityDeregisterCmd = &cobra.Command{
		Use:   "deregister",
		Short: "Remove account entity from registry",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			if npa.Account == nil {
				cobra.CheckErr("no accounts configured in your wallet")
			}

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			// Prepare transaction.
			tx := registry.NewDeregisterEntityTx(0, nil)

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa.ParaTime, conn, sigTx, nil, nil)
		},
	}
)

func init() {
	entityInitCmd.Flags().StringVarP(&entityOutputFile, "output-file", "o", "", "output entity descriptor into specified JSON file")
	entityInitCmd.Flags().AddFlagSet(common.AccountFlag)
	entityInitCmd.Flags().AddFlagSet(common.YesFlag)

	entityRegisterCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	entityRegisterCmd.Flags().AddFlagSet(common.TxFlags)

	entityDeregisterCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	entityDeregisterCmd.Flags().AddFlagSet(common.TxFlags)

	entityCmd.AddCommand(entityInitCmd)
	entityCmd.AddCommand(entityRegisterCmd)
	entityCmd.AddCommand(entityDeregisterCmd)
}
