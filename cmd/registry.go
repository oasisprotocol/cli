package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/entity"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	registryCmd = &cobra.Command{
		Use:   "registry",
		Short: "Registry operations",
	}

	registryEntityRegisterCmd = &cobra.Command{
		Use:   "entity-register <entity.json>",
		Short: "Register a new entity or update an existing one",
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

			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, nil, nil)
		},
	}

	registryEntityDeregisterCmd = &cobra.Command{
		Use:   "entity-deregister",
		Short: "Deregister an existing entity",
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

			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, nil, nil)
		},
	}

	registryNodeUnfreezeCmd = &cobra.Command{
		Use:   "node-unfreeze <node-id>",
		Short: "Unfreeze a frozen node",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			rawNodeID := args[0]

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

			// Parse node identifier.
			var nodeID signature.PublicKey
			err := nodeID.UnmarshalText([]byte(rawNodeID))
			cobra.CheckErr(err)

			// Prepare transaction.
			tx := registry.NewUnfreezeNodeTx(0, nil, &registry.UnfreezeNode{
				NodeID: nodeID,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, nil, nil)
		},
	}

	registryRuntimeRegisterCmd = &cobra.Command{
		Use:   "runtime-register <descriptor.json>",
		Short: "Register a new runtime or update an existing one",
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

			// Load runtime descriptor.
			rawDescriptor, err := os.ReadFile(filename)
			cobra.CheckErr(err)

			// Parse runtime descriptor.
			var descriptor registry.Runtime
			if err = json.Unmarshal(rawDescriptor, &descriptor); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed runtime descriptor: %w", err))
			}

			// Prepare transaction.
			tx := registry.NewRegisterRuntimeTx(0, nil, &descriptor)

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, nil, nil)
		},
	}

	registryShowCmd = &cobra.Command{
		Use:   "show { <id> | entities | nodes | runtimes | validators }",
		Short: "Show registry entry by id (or show all entries of a specified kind)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			id, err := parseIdentifier(npa, args[0])
			cobra.CheckErr(err)

			// Establish connection with the target network.
			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			consensusConn := conn.Consensus()
			registryConn := consensusConn.Registry()

			// Figure out the height to use if "latest".
			height, err := common.GetActualHeight(
				ctx,
				consensusConn,
			)
			cobra.CheckErr(err)

			// This command just takes a brute-force "do-what-I-mean" approach
			// and queries everything it can till it finds what the user is
			// looking for.

			prettyPrint := func(b interface{}) error {
				data, err := json.MarshalIndent(b, "", "  ")
				if err != nil {
					return err
				}
				fmt.Printf("%s\n", data)
				return nil
			}

			switch v := id.(type) {
			case signature.PublicKey:
				idQuery := &registry.IDQuery{
					Height: height,
					ID:     v,
				}

				if entity, err := registryConn.GetEntity(ctx, idQuery); err == nil {
					err = prettyPrint(entity)
					cobra.CheckErr(err)
					return
				}

				if node, err := registryConn.GetNode(ctx, idQuery); err == nil {
					err = prettyPrint(node)
					cobra.CheckErr(err)
					return
				}

				nsQuery := &registry.GetRuntimeQuery{
					Height: height,
				}
				copy(nsQuery.ID[:], v[:])

				if runtime, err := registryConn.GetRuntime(ctx, nsQuery); err == nil {
					err = prettyPrint(runtime)
					cobra.CheckErr(err)
					return
				}
			case *types.Address:
				addr := staking.Address(*v)

				entities, err := registryConn.GetEntities(ctx, height)
				cobra.CheckErr(err) // If this doesn't work the other large queries won't either.
				for _, entity := range entities {
					if staking.NewAddress(entity.ID).Equal(addr) {
						err = prettyPrint(entity)
						cobra.CheckErr(err)
						return
					}
				}

				nodes, err := registryConn.GetNodes(ctx, height)
				cobra.CheckErr(err)
				for _, node := range nodes {
					if staking.NewAddress(node.ID).Equal(addr) {
						err = prettyPrint(node)
						cobra.CheckErr(err)
						return
					}
				}

				// Probably don't need to bother querying the runtimes by address.
			case registrySelector:
				switch v {
				case selEntities:
					entities, err := registryConn.GetEntities(ctx, height)
					cobra.CheckErr(err)
					for _, entity := range entities {
						err = prettyPrint(entity)
						cobra.CheckErr(err)
					}
					return
				case selNodes:
					nodes, err := registryConn.GetNodes(ctx, height)
					cobra.CheckErr(err)
					for _, node := range nodes {
						err = prettyPrint(node)
						cobra.CheckErr(err)
					}
					return
				case selRuntimes:
					runtimes, err := registryConn.GetRuntimes(ctx, &registry.GetRuntimesQuery{
						Height:           height,
						IncludeSuspended: true,
					})
					cobra.CheckErr(err)
					for _, runtime := range runtimes {
						err = prettyPrint(runtime)
						cobra.CheckErr(err)
					}
					return
				case selValidators:
					// Yes, this is a scheduler query, not a registry query
					// but this also is a reasonable place for this.
					schedulerConn := consensusConn.Scheduler()
					validators, err := schedulerConn.GetValidators(ctx, height)
					cobra.CheckErr(err)
					for _, validator := range validators {
						err = prettyPrint(validator)
						cobra.CheckErr(err)
					}
					return
				default:
					// Should never happen.
				}
			}

			cobra.CheckErr(fmt.Errorf("id '%s' not found", id))
		},
	}
)

type registrySelector int

const (
	selInvalid registrySelector = iota
	selEntities
	selNodes
	selRuntimes
	selValidators
)

func selectorFromString(s string) registrySelector {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "entities":
		return selEntities
	case "nodes":
		return selNodes
	case "runtimes", "paratimes":
		return selRuntimes
	case "validators":
		return selValidators
	}
	return selInvalid
}

func parseIdentifier(
	npa *common.NPASelection,
	s string,
) (interface{}, error) { // TODO: Use `any`
	if sel := selectorFromString(s); sel != selInvalid {
		return sel, nil
	}

	addr, _, err := helpers.ResolveAddress(npa.Network, s)
	if err == nil {
		return addr, nil
	}

	var pk signature.PublicKey
	if err = pk.UnmarshalText([]byte(s)); err == nil {
		return pk, nil
	}
	if err = pk.UnmarshalHex(s); err == nil {
		return pk, nil
	}

	return nil, fmt.Errorf("unrecognized id: '%s'", s)
}

func init() {
	registryEntityRegisterCmd.Flags().AddFlagSet(common.SelectorFlags)
	registryEntityRegisterCmd.Flags().AddFlagSet(common.TransactionFlags)

	registryEntityDeregisterCmd.Flags().AddFlagSet(common.SelectorFlags)
	registryEntityDeregisterCmd.Flags().AddFlagSet(common.TransactionFlags)

	registryNodeUnfreezeCmd.Flags().AddFlagSet(common.SelectorFlags)
	registryNodeUnfreezeCmd.Flags().AddFlagSet(common.TransactionFlags)

	registryRuntimeRegisterCmd.Flags().AddFlagSet(common.SelectorFlags)
	registryRuntimeRegisterCmd.Flags().AddFlagSet(common.TransactionFlags)

	registryShowCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	registryShowCmd.Flags().AddFlagSet(common.HeightFlag)

	registryCmd.AddCommand(registryEntityRegisterCmd)
	registryCmd.AddCommand(registryEntityDeregisterCmd)
	registryCmd.AddCommand(registryNodeUnfreezeCmd)
	registryCmd.AddCommand(registryRuntimeRegisterCmd)
	registryCmd.AddCommand(registryShowCmd)
}
