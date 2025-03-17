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
	"gopkg.in/yaml.v3"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/contracts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	contractInstantiatePolicy string
	contractUpgradesPolicy    string
	contractTokens            []string
	contractStorageDumpKind   string
	contractStorageDumpLimit  uint64
	contractStorageDumpOffset uint64

	contractCmd = &cobra.Command{
		Use:     "contract",
		Short:   "WebAssembly smart contracts operations",
		Aliases: []string{"c", "contracts"},
	}

	contractShowCmd = &cobra.Command{
		Use:   "show <instance-id>",
		Short: "Show information about instantiated contract",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			strInstanceID := args[0]

			npa.MustHaveParaTime()

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

	contractShowCodeCmd = &cobra.Command{
		Use:   "show-code <code-id>",
		Short: "Show information about uploaded contract code",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			strCodeID := args[0]

			npa.MustHaveParaTime()

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

	contractStorageCmd = &cobra.Command{
		Use:   "storage",
		Short: "WebAssembly smart contracts storage operations",
	}

	contractStorageDumpCmd = &cobra.Command{
		Use:   "dump <instance-id>",
		Short: "Dump contract store",
		Long: `Dump public or confidential contract store in JSON. Valid UTF-8 keys in the result set will be
encoded as strings, or otherwise as Base64.`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			strInstanceID := args[0]

			npa.MustHaveParaTime()

			instanceID, err := strconv.ParseUint(strInstanceID, 10, 64)
			cobra.CheckErr(err)

			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			var storeKind contracts.StoreKind
			cobra.CheckErr(storeKind.UnmarshalText([]byte(contractStorageDumpKind)))

			res, err := conn.Runtime(npa.ParaTime).Contracts.InstanceRawStorage(
				ctx,
				client.RoundLatest,
				contracts.InstanceID(instanceID),
				storeKind,
				contractStorageDumpLimit,
				contractStorageDumpOffset,
			)
			cobra.CheckErr(err)

			fmt.Printf(
				"Showing %d %s record(s) of contract %d:\n",
				len(res.Items),
				contractStorageDumpKind,
				instanceID,
			)
			common.JSONPrintKeyValueTuple(res.Items)
		},
	}

	contractStorageGetCmd = &cobra.Command{
		Use:   "get <instance-id> <key>",
		Short: "Print value for given key in public contract store",
		Long: `Print value for the given key in the public contract store in JSON format. The given key can be
a string or Base64-encoded. Valid UTF-8 keys in the result set will be encoded as strings, or
otherwise as Base64.`,
		Args: cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			strInstanceID := args[0]
			strKey := args[1]

			npa.MustHaveParaTime()

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

	contractDumpCodeCmd = &cobra.Command{
		Use:   "dump-code <code-id>",
		Short: "Dump WebAssembly smart contract code",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			strCodeID := args[0]

			npa.MustHaveParaTime()

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

	contractUploadCmd = &cobra.Command{
		Use:   "upload <contract.wasm> [--instantiate-policy POLICY]",
		Short: "Upload WebAssembly smart contract",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			wasmFilename := args[0]

			npa.MustHaveAccount()
			npa.MustHaveParaTime()

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
			instantiatePolicy := parsePolicy(npa.Network, npa.Account, contractInstantiatePolicy)

			// Prepare transaction.
			tx := contracts.NewUploadTx(nil, &contracts.Upload{
				ABI:               contracts.ABIOasisV1,
				InstantiatePolicy: *instantiatePolicy,
				Code:              contracts.CompressCode(wasmData),
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			var result contracts.UploadResult
			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, &result) {
				return
			}

			fmt.Printf("Code ID: %d\n", result.ID)
		},
	}

	contractInstantiateCmd = &cobra.Command{
		Use:     "instantiate <code-id> <data-yaml> [--tokens TOKENS] [--upgrades-policy POLICY]",
		Aliases: []string{"inst"},
		Short:   "Instantiate WebAssembly smart contract",
		Args:    cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			strCodeID := args[0]
			strData := args[1]

			npa.MustHaveAccount()
			npa.MustHaveParaTime()

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
			upgradesPolicy := parsePolicy(npa.Network, npa.Account, contractUpgradesPolicy)

			// Parse tokens that should be sent to the contract.
			tokens := parseTokens(npa.ParaTime, contractTokens)

			// Prepare transaction.
			tx := contracts.NewInstantiateTx(nil, &contracts.Instantiate{
				CodeID:         contracts.CodeID(codeID),
				UpgradesPolicy: *upgradesPolicy,
				Data:           cbor.Marshal(data),
				Tokens:         tokens,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			var result contracts.InstantiateResult
			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, &result) {
				return
			}

			fmt.Printf("Instance ID: %d\n", result.ID)
		},
	}

	contractCallCmd = &cobra.Command{
		Use:   "call <instance-id> <data-yaml> [--tokens TOKENS]",
		Short: "Call WebAssembly smart contract",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			strInstanceID := args[0]
			strData := args[1]

			npa.MustHaveAccount()
			npa.MustHaveParaTime()

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
			tokens := parseTokens(npa.ParaTime, contractTokens)

			// Prepare transaction.
			tx := contracts.NewCallTx(nil, &contracts.Call{
				ID:     contracts.InstanceID(instanceID),
				Data:   cbor.Marshal(data),
				Tokens: tokens,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			var result contracts.CallResult
			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, &result) {
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

	contractChangeUpgradePolicyCmd = &cobra.Command{
		Use:   "change-upgrade-policy <instance-id> <policy>",
		Short: "Change WebAssembly smart contract upgrade policy",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()
			strInstanceID := args[0]
			strPolicy := args[1]

			npa.MustHaveAccount()
			npa.MustHaveParaTime()

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
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil)
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
		address, _, err := common.ResolveLocalAccountOrAddress(net, policy)
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
	contractShowCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractShowCodeCmd.Flags().AddFlagSet(common.SelectorFlags)

	contractDumpCodeCmd.Flags().AddFlagSet(common.SelectorFlags)

	contractsUploadFlags := flag.NewFlagSet("", flag.ContinueOnError)
	contractsUploadFlags.StringVar(&contractInstantiatePolicy, "instantiate-policy", "everyone", "contract instantiation policy")

	contractUploadCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractUploadCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	contractUploadCmd.Flags().AddFlagSet(contractsUploadFlags)

	contractsCallFlags := flag.NewFlagSet("", flag.ContinueOnError)
	contractsCallFlags.StringSliceVar(&contractTokens, "tokens", []string{}, "token amounts to send to a contract")

	contractsInstantiateFlags := flag.NewFlagSet("", flag.ContinueOnError)
	contractsInstantiateFlags.StringVar(&contractUpgradesPolicy, "upgrades-policy", "owner", "contract upgrades policy")

	contractInstantiateCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractInstantiateCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	contractInstantiateCmd.Flags().AddFlagSet(contractsInstantiateFlags)
	contractInstantiateCmd.Flags().AddFlagSet(contractsCallFlags)

	contractCallCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractCallCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	contractCallCmd.Flags().AddFlagSet(contractsCallFlags)

	contractChangeUpgradePolicyCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractChangeUpgradePolicyCmd.Flags().AddFlagSet(common.RuntimeTxFlags)

	contractsStorageDumpCmdFlags := flag.NewFlagSet("", flag.ContinueOnError)
	contractsStorageDumpCmdFlags.StringVar(&contractStorageDumpKind, "kind", "public",
		fmt.Sprintf("store kind [%s]", strings.Join([]string{
			contracts.StoreKindPublicName,
			contracts.StoreKindConfidentialName,
		}, ", ")),
	)
	contractsStorageDumpCmdFlags.Uint64Var(&contractStorageDumpLimit, "limit", 0, "result set limit")
	contractsStorageDumpCmdFlags.Uint64Var(&contractStorageDumpOffset, "offset", 0, "result set offset")
	contractStorageDumpCmd.Flags().AddFlagSet(common.SelectorFlags)
	contractStorageDumpCmd.Flags().AddFlagSet(contractsStorageDumpCmdFlags)

	contractStorageGetCmd.Flags().AddFlagSet(common.SelectorFlags)

	contractStorageCmd.AddCommand(contractStorageDumpCmd)
	contractStorageCmd.AddCommand(contractStorageGetCmd)

	contractCmd.AddCommand(contractShowCmd)
	contractCmd.AddCommand(contractShowCodeCmd)
	contractCmd.AddCommand(contractStorageCmd)
	contractCmd.AddCommand(contractDumpCodeCmd)
	contractCmd.AddCommand(contractUploadCmd)
	contractCmd.AddCommand(contractInstantiateCmd)
	contractCmd.AddCommand(contractCallCmd)
	contractCmd.AddCommand(contractChangeUpgradePolicyCmd)
}
