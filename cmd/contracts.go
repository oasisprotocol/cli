package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"gopkg.in/yaml.v2"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/contracts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

var (
	contractsInstantiatePolicy string
	contractsUpgradesPolicy    string
	contractsTokens            []string
	contractsStorageDumpKind   string
	contractsStorageDumpLimit  uint64
	contractsStorageDumpOffset uint64

	contractsCmd = &cobra.Command{
		Use:   "contracts",
		Short: "WebAssembly smart contracts operations",
	}

	contractsShowCmd = &cobra.Command{
		Use:   "show <instance-id>",
		Short: "Show information about instantiated contract",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			strInstanceID := args[0]

			if npa.ParaTime == nil {
				cobra.CheckErr("no paratimes configured")
			}

			instanceID, err := strconv.ParseUint(strInstanceID, 10, 64)
			cobra.CheckErr(err)

			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			inst, err := conn.Runtime(npa.ParaTime).Contracts.Instance(ctx, client.RoundLatest, contracts.InstanceID(instanceID))
			cobra.CheckErr(err)

			fmt.Printf("ID:              %d\n", inst.ID)
			fmt.Printf("Code ID:         %d\n", inst.CodeID)
			fmt.Printf("Creator:         %s\n", inst.Creator)
			fmt.Printf("Upgrades policy: %s\n", formatPolicy(&inst.UpgradesPolicy))
		},
	}

	contractsShowCodeCmd = &cobra.Command{
		Use:   "show-code <code-id>",
		Short: "Show information about uploaded contract code",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			strCodeID := args[0]

			if npa.ParaTime == nil {
				cobra.CheckErr("no paratimes configured")
			}

			codeID, err := strconv.ParseUint(strCodeID, 10, 64)
			cobra.CheckErr(err)

			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			code, err := conn.Runtime(npa.ParaTime).Contracts.Code(ctx, client.RoundLatest, contracts.CodeID(codeID))
			cobra.CheckErr(err)

			fmt.Printf("ID:                 %d\n", code.ID)
			fmt.Printf("Hash:               %s\n", code.Hash)
			fmt.Printf("ABI:                %s (sv: %d)\n", code.ABI, code.ABISubVersion)
			fmt.Printf("Uploader:           %s\n", code.Uploader)
			fmt.Printf("Instantiate policy: %s\n", formatPolicy(&code.InstantiatePolicy))
		},
	}

	contractsStorageCmd = &cobra.Command{
		Use:   "storage",
		Short: "WebAssembly smart contracts storage operations",
	}

	contractsStorageDumpCmd = &cobra.Command{
		Use:   "dump <instance-id>",
		Short: "Dump contract store",
		Long: `Dump public or confidential contract store in JSON. Valid UTF-8 keys in the result set will be
encoded as strings, or otherwise as Base64.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			strInstanceID := args[0]

			if npa.ParaTime == nil {
				cobra.CheckErr("no paratimes configured")
			}

			instanceID, err := strconv.ParseUint(strInstanceID, 10, 64)
			cobra.CheckErr(err)

			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			var storeKind contracts.StoreKind
			cobra.CheckErr(storeKind.UnmarshalText([]byte(contractsStorageDumpKind)))

			res, err := conn.Runtime(npa.ParaTime).Contracts.InstanceRawStorage(
				ctx,
				client.RoundLatest,
				contracts.InstanceID(instanceID),
				storeKind,
				contractsStorageDumpLimit,
				contractsStorageDumpOffset,
			)
			cobra.CheckErr(err)

			fmt.Printf(
				"Showing %d %s record(s) of contract %d:\n",
				len(res.Items),
				contractsStorageDumpKind,
				instanceID,
			)
			common.JSONPrintKeyValueTuple(res.Items)
		},
	}

	contractsStorageGetCmd = &cobra.Command{
		Use:   "get <instance-id> <key>",
		Short: "Print value for given key in public contract store",
		Long: `Print value for the given key in the public contract store in JSON format. The given key can be
a string or Base64-encoded. Valid UTF-8 keys in the result set will be encoded as strings, or
otherwise as Base64.`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			strInstanceID := args[0]
			strKey := args[1]

			if npa.ParaTime == nil {
				cobra.CheckErr("no paratimes configured")
			}

			instanceID, err := strconv.ParseUint(strInstanceID, 10, 64)
			cobra.CheckErr(err)

			// Try parsing the query key as Base64-encoded value. This allows users to query binary
			// keys. If decoding fails, fallback to original value.
			var key []byte
			if err = json.Unmarshal([]byte(fmt.Sprintf("\"%s\"", strKey)), &key); err != nil {
				key = []byte(strKey)
			}

			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			res, err := conn.Runtime(npa.ParaTime).Contracts.InstanceStorage(
				ctx,
				client.RoundLatest,
				contracts.InstanceID(instanceID),
				key,
			)
			cobra.CheckErr(err)

			var storageCell interface{}
			err = cbor.Unmarshal(res.Value, &storageCell)
			if err != nil {
				// Value is not CBOR, use raw value instead.
				storageCell = res.Value
			}
			fmt.Printf("%s\n", common.JSONMarshalUniversalValue(storageCell))
		},
	}

	contractsDumpCodeCmd = &cobra.Command{
		Use:   "dump-code <code-id>",
		Short: "Dump WebAssembly smart contract code",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			strCodeID := args[0]

			if npa.ParaTime == nil {
				cobra.CheckErr("no paratimes configured")
			}

			codeID, err := strconv.ParseUint(strCodeID, 10, 64)
			cobra.CheckErr(err)

			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			// Fetch WASM contract code, if supported.
			codeStorage, err := conn.Runtime(npa.ParaTime).Contracts.CodeStorage(
				ctx,
				client.RoundLatest,
				contracts.CodeID(codeID),
			)
			cobra.CheckErr(err)

			os.Stdout.Write(codeStorage.Code)
		},
	}

	contractsUploadCmd = &cobra.Command{
		Use:   "upload <contract.wasm> [--instantiate-policy POLICY]",
		Short: "Upload WebAssembly smart contract",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			wasmFilename := args[0]

			if npa.Account == nil {
				cobra.CheckErr("no accounts configured in your wallet")
			}
			if npa.ParaTime == nil {
				cobra.CheckErr("no paratimes configured")
			}

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			// Read WASM from file.
			wasmData, err := os.ReadFile(wasmFilename)
			cobra.CheckErr(err)

			// Parse instantiation policy.
			instantiatePolicy := parsePolicy(npa.Network, npa.Account, contractsInstantiatePolicy)

			// Prepare transaction.
			tx := contracts.NewUploadTx(nil, &contracts.Upload{
				ABI:               contracts.ABIOasisV1,
				InstantiatePolicy: *instantiatePolicy,
				Code:              contracts.CompressCode(wasmData),
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			var result contracts.UploadResult
			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, meta, &result)

			if txCfg.Offline {
				return
			}

			fmt.Printf("Code ID: %d\n", result.ID)
		},
	}

	contractsInstantiateCmd = &cobra.Command{
		Use:     "instantiate <code-id> <data-yaml> [--tokens TOKENS] [--upgrades-policy POLICY]",
		Aliases: []string{"inst"},
		Short:   "Instantiate WebAssembly smart contract",
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			strCodeID := args[0]
			strData := args[1]

			if npa.Account == nil {
				cobra.CheckErr("no accounts configured in your wallet")
			}
			if npa.ParaTime == nil {
				cobra.CheckErr("no paratimes configured")
			}

			codeID, err := strconv.ParseUint(strCodeID, 10, 64)
			cobra.CheckErr(err)

			// Parse instantiation arguments.
			data := parseData(strData)

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			// Parse upgrades policy.
			upgradesPolicy := parsePolicy(npa.Network, npa.Account, contractsUpgradesPolicy)

			// Parse tokens that should be sent to the contract.
			tokens := parseTokens(npa.ParaTime, contractsTokens)

			// Prepare transaction.
			tx := contracts.NewInstantiateTx(nil, &contracts.Instantiate{
				CodeID:         contracts.CodeID(codeID),
				UpgradesPolicy: *upgradesPolicy,
				Data:           cbor.Marshal(data),
				Tokens:         tokens,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			var result contracts.InstantiateResult
			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, meta, &result)

			if txCfg.Offline {
				return
			}

			fmt.Printf("Instance ID: %d\n", result.ID)
		},
	}

	contractsCallCmd = &cobra.Command{
		Use:   "call <instance-id> <data-yaml> [--tokens TOKENS]",
		Short: "Call WebAssembly smart contract",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			strInstanceID := args[0]
			strData := args[1]

			if npa.Account == nil {
				cobra.CheckErr("no accounts configured in your wallet")
			}
			if npa.ParaTime == nil {
				cobra.CheckErr("no paratimes configured")
			}

			instanceID, err := strconv.ParseUint(strInstanceID, 10, 64)
			cobra.CheckErr(err)

			// Parse call arguments.
			data := parseData(strData)

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			// Parse tokens that should be sent to the contract.
			tokens := parseTokens(npa.ParaTime, contractsTokens)

			// Prepare transaction.
			tx := contracts.NewCallTx(nil, &contracts.Call{
				ID:     contracts.InstanceID(instanceID),
				Data:   cbor.Marshal(data),
				Tokens: tokens,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			var result contracts.CallResult
			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, meta, &result)

			if txCfg.Offline {
				return
			}

			fmt.Printf("Call result:\n")

			var decResult interface{}
			err = cbor.Unmarshal(result, &decResult)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to unmarshal call result: %w", err))
			}

			formatted, err := yaml.Marshal(decResult)
			cobra.CheckErr(err)
			fmt.Println(string(formatted))
		},
	}

	contractsChangeUpgradePolicyCmd = &cobra.Command{
		Use:   "change-upgrade-policy <instance-id> <policy>",
		Short: "Change WebAssembly smart contract upgrade policy",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			strInstanceID := args[0]
			strPolicy := args[1]

			if npa.Account == nil {
				cobra.CheckErr("no accounts configured in your wallet")
			}
			if npa.ParaTime == nil {
				cobra.CheckErr("no paratimes configured")
			}

			instanceID, err := strconv.ParseUint(strInstanceID, 10, 64)
			cobra.CheckErr(err)

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			// Parse upgrades policy.
			upgradesPolicy := parsePolicy(npa.Network, npa.Account, strPolicy)

			// Prepare transaction.
			tx := contracts.NewChangeUpgradePolicyTx(nil, &contracts.ChangeUpgradePolicy{
				ID:             contracts.InstanceID(instanceID),
				UpgradesPolicy: *upgradesPolicy,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, meta, nil)
		},
	}
)

func formatPolicy(policy *contracts.Policy) string {
	switch {
	case policy.Nobody != nil:
		return "nobody"
	case policy.Address != nil:
		return fmt.Sprintf("address:%s", policy.Address.String())
	case policy.Everyone != nil:
		return "everyone"
	default:
		return "[unknown]"
	}
}

func parsePolicy(net *config.Network, wallet *cliConfig.Account, policy string) *contracts.Policy {
	switch {
	case policy == "nobody":
		return &contracts.Policy{Nobody: &struct{}{}}
	case policy == "everyone":
		return &contracts.Policy{Everyone: &struct{}{}}
	case policy == "owner":
		address := wallet.GetAddress()
		return &contracts.Policy{Address: &address}
	case strings.HasPrefix(policy, "address:"):
		policy = strings.TrimPrefix(policy, "address:")
		address, err := common.ResolveLocalAccountOrAddress(net, policy)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("malformed address in policy: %w", err))
		}
		return &contracts.Policy{Address: address}
	default:
		cobra.CheckErr(fmt.Sprintf("invalid policy: %s", policy))
	}
	return nil
}

func parseData(data string) interface{} {
	var result interface{}
	if len(data) > 0 {
		err := yaml.Unmarshal([]byte(data), &result)
		cobra.CheckErr(err)
	}
	return result
}

func parseTokens(pt *config.ParaTime, tokens []string) []types.BaseUnits {
	result := []types.BaseUnits{}
	for _, raw := range tokens {
		// TODO: Support parsing denominations.
		amount, err := helpers.ParseParaTimeDenomination(pt, raw, types.NativeDenomination)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("malformed token amount: %w", err))
		}
		result = append(result, *amount)
	}
	return result
}

func init() {
	contractsShowCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractsShowCodeCmd.Flags().AddFlagSet(common.SelectorFlags)

	contractsDumpCodeCmd.Flags().AddFlagSet(common.SelectorFlags)

	contractsUploadFlags := flag.NewFlagSet("", flag.ContinueOnError)
	contractsUploadFlags.StringVar(&contractsInstantiatePolicy, "instantiate-policy", "everyone", "contract instantiation policy")

	contractsUploadCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractsUploadCmd.Flags().AddFlagSet(common.TransactionFlags)
	contractsUploadCmd.Flags().AddFlagSet(contractsUploadFlags)

	contractsCallFlags := flag.NewFlagSet("", flag.ContinueOnError)
	contractsCallFlags.StringSliceVar(&contractsTokens, "tokens", []string{}, "token amounts to send to a contract")

	contractsInstantiateFlags := flag.NewFlagSet("", flag.ContinueOnError)
	contractsInstantiateFlags.StringVar(&contractsUpgradesPolicy, "upgrades-policy", "owner", "contract upgrades policy")

	contractsInstantiateCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractsInstantiateCmd.Flags().AddFlagSet(common.TransactionFlags)
	contractsInstantiateCmd.Flags().AddFlagSet(contractsInstantiateFlags)
	contractsInstantiateCmd.Flags().AddFlagSet(contractsCallFlags)

	contractsCallCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractsCallCmd.Flags().AddFlagSet(common.TransactionFlags)
	contractsCallCmd.Flags().AddFlagSet(contractsCallFlags)

	contractsChangeUpgradePolicyCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractsChangeUpgradePolicyCmd.Flags().AddFlagSet(common.TransactionFlags)

	contractsStorageDumpCmdFlags := flag.NewFlagSet("", flag.ContinueOnError)
	contractsStorageDumpCmdFlags.StringVar(&contractsStorageDumpKind, "kind", "public",
		fmt.Sprintf("store kind [%s]", strings.Join([]string{
			contracts.StoreKindPublicName,
			contracts.StoreKindConfidentialName,
		}, ", ")),
	)
	contractsStorageDumpCmdFlags.Uint64Var(&contractsStorageDumpLimit, "limit", 0, "result set limit")
	contractsStorageDumpCmdFlags.Uint64Var(&contractsStorageDumpOffset, "offset", 0, "result set offset")
	contractsStorageDumpCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractsStorageDumpCmd.Flags().AddFlagSet(contractsStorageDumpCmdFlags)

	contractsStorageGetCmd.Flags().AddFlagSet(common.SelectorFlags)

	contractsStorageCmd.AddCommand(contractsStorageDumpCmd)
	contractsStorageCmd.AddCommand(contractsStorageGetCmd)

	contractsCmd.AddCommand(contractsShowCmd)
	contractsCmd.AddCommand(contractsShowCodeCmd)
	contractsCmd.AddCommand(contractsStorageCmd)
	contractsCmd.AddCommand(contractsDumpCodeCmd)
	contractsCmd.AddCommand(contractsUploadCmd)
	contractsCmd.AddCommand(contractsInstantiateCmd)
	contractsCmd.AddCommand(contractsCallCmd)
	contractsCmd.AddCommand(contractsChangeUpgradePolicyCmd)
}
