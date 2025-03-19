package account

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	metadataRegistry "github.com/oasisprotocol/metadata-registry-tools"
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
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			npa.MustHaveAccount()

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
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			filename := args[0]

			npa.MustHaveAccount()

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

			common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
		},
	}

	entityDeregisterCmd = &cobra.Command{
		Use:   "deregister",
		Short: "Remove account entity from registry",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			npa.MustHaveAccount()

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

			common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
		},
	}

	registryPath            string
	entityMetadataUpdateCmd = &cobra.Command{
		Use:   "metadata-update <entity.json>",
		Short: "Update account entity metadata in registry",
		Long:  "Update your account entity metadata in the network registry.",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			npa.MustHaveAccount()

			var fsRegistry metadataRegistry.MutableProvider
			switch registryPath {
			case "":
				// Setup filesystem registry in the current directory.
				common.Confirm("Metadata registry directory not specified, initializing registry in current directory", "not updating metadata")
				wd, err := os.Getwd()
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to get current working directory: %w", err))
				}
				fsRegistry, err = metadataRegistry.NewFilesystemPathProvider(wd)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to initialize metadata registry in current directory: %w", err))
				}
			default:
				// Setup filesystem registry in the specified directory.
				var err error
				fsRegistry, err = metadataRegistry.NewFilesystemPathProvider(registryPath)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to initialize metadata registry: '%s': %w", registryPath, err))
				}
			}

			// Open and parse the passed entity metadata file.
			rawMetadata, err := os.ReadFile(args[0])
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to read entity metadata file: %w", err))
			}
			var metadata metadataRegistry.EntityMetadata
			if err = json.Unmarshal(rawMetadata, &metadata); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to parse serialized entity metadata: %w", err))
			}

			// Load the account and ensure it corresponds to the entity.
			acc := common.LoadAccount(cfg, npa.AccountName)
			signer := acc.ConsensusSigner()
			if signer == nil {
				cobra.CheckErr(fmt.Errorf("account '%s' does not support signing consensus transactions", npa.AccountName))
			}

			if err = metadata.ValidateBasic(); err != nil {
				cobra.CheckErr(fmt.Errorf("provided entity metadata is invalid: %w", err))
			}

			metadata.PrettyPrint(context.Background(), "  ", os.Stdout)
			common.Confirm("You are about to sign the above metadata descriptor", "not singing metadata descriptor")

			signed, err := metadataRegistry.SignEntityMetadata(signer, &metadata)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to sign entity metadata: %w", err))
			}

			if err = fsRegistry.UpdateEntity(signed); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update entity metadata: %w", err))
			}

			fmt.Printf("Successfully updated entity metadata.\n")
		},
	}
)

func init() {
	entityInitCmd.Flags().StringVarP(&entityOutputFile, "output-file", "o", "", "output entity descriptor into specified JSON file")
	entityInitCmd.Flags().AddFlagSet(common.AccountFlag)
	entityInitCmd.Flags().AddFlagSet(common.AnswerYesFlag)

	entityRegisterCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	entityRegisterCmd.Flags().AddFlagSet(common.TxFlags)

	entityDeregisterCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	entityDeregisterCmd.Flags().AddFlagSet(common.TxFlags)

	entityMetadataUpdateCmd.Flags().StringVarP(&registryPath, "registry-dir", "r", "", "path to the metadata registry directory")
	entityMetadataUpdateCmd.Flags().AddFlagSet(common.AccountFlag)
	entityMetadataUpdateCmd.Flags().AddFlagSet(common.AnswerYesFlag)

	entityCmd.AddCommand(entityInitCmd)
	entityCmd.AddCommand(entityRegisterCmd)
	entityCmd.AddCommand(entityDeregisterCmd)
	entityCmd.AddCommand(entityMetadataUpdateCmd)
}
