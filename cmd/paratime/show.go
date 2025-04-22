package paratime

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	runtimeTx "github.com/oasisprotocol/oasis-core/go/runtime/transaction"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/contracts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

type propertySelector int

const (
	selInvalid propertySelector = iota
	selParameters
	selEvents
	selRoundLatest = "latest"
)

var eventDecoders = []func(*types.Event) ([]client.DecodedEvent, error){
	accounts.DecodeEvent,
	consensusaccounts.DecodeEvent,
	contracts.DecodeEvent,
	core.DecodeEvent,
	evm.DecodeEvent,
	rofl.DecodeEvent,
}

var (
	selectedRound uint64

	showCmd = &cobra.Command{
		Use:     "show { <round> [ <tx-index> | <tx-hash> ] | parameters | events }",
		Short:   "Show information about a ParaTime block, its transactions, events or other parameters",
		Long:    "Show ParaTime-specific information about a given block round, (optionally) its transactions or other information. Use \"latest\" to use the last round.",
		Aliases: []string{"s"},
		Args:    cobra.RangeArgs(1, 2),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			npa.MustHaveParaTime()

			p, err := parseBlockNum(args[0])
			cobra.CheckErr(err)

			var (
				//			err error
				//			blkNum  uint64
				txIndex int
				txHash  hash.Hash
			)

			if len(args) >= 2 {
				txIndexOrHash := args[1]

				txIndex, err = strconv.Atoi(txIndexOrHash)
				if err != nil {
					txIndex = -1
					if err = txHash.UnmarshalHex(txIndexOrHash); err != nil {
						cobra.CheckErr(fmt.Errorf("malformed tx hash: %w", err))
					}
				}
			}

			// Establish connection with the target network.
			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			if common.OutputFormat() == common.FormatText {
				fmt.Printf("Network:        %s", npa.NetworkName)
				if len(npa.Network.Description) > 0 {
					fmt.Printf(" (%s)", npa.Network.Description)
				}
				fmt.Println()
				fmt.Printf("ParaTime:       %s", npa.ParaTimeName)
				if len(npa.ParaTime.Description) > 0 {
					fmt.Printf(" (%s)", npa.ParaTime.Description)
				}
				fmt.Println()
			}

			// Runtime layer.
			rt := conn.Runtime(npa.ParaTime)

			switch v := p.(type) {
			case uint64:
				blkNum := v

				blk, err := rt.GetBlock(ctx, blkNum)
				cobra.CheckErr(err)

				fmt.Printf("Round:          %d\n", blk.Header.Round)
				fmt.Printf("Version:        %d\n", blk.Header.Version)
				fmt.Printf("Namespace:      %s\n", blk.Header.Namespace)

				// TODO: Fix when timestamp has a String method.
				ts, _ := blk.Header.Timestamp.MarshalText()
				fmt.Printf("Timestamp:      %s\n", string(ts))

				// TODO: Fix when type has a String method.
				fmt.Printf("Type:           %d\n", blk.Header.HeaderType)
				fmt.Printf("Previous:       %s\n", blk.Header.PreviousHash)
				fmt.Printf("I/O root:       %s\n", blk.Header.IORoot)
				fmt.Printf("State root:     %s\n", blk.Header.StateRoot)
				fmt.Printf("Messages (out): %s\n", blk.Header.MessagesHash)
				fmt.Printf("Messages (in):  %s\n", blk.Header.InMessagesHash)

				txs, err := rt.GetTransactionsWithResults(ctx, blk.Header.Round)
				cobra.CheckErr(err)

				fmt.Printf("Transactions:   %d\n", len(txs))

				evs, err := rt.GetEventsRaw(ctx, blkNum)
				cobra.CheckErr(err)
				if len(evs) > 0 {
					// Check if there were any block events emitted.
					var blockEvs []*types.Event
					for _, ev := range evs {
						if ev.TxHash.Equal(&runtimeTx.TagBlockTxHash) {
							blockEvs = append(blockEvs, ev)
						}
					}

					if numEvents := len(blockEvs); numEvents > 0 {
						fmt.Println()
						fmt.Printf("=== Block events ===\n")
						fmt.Printf("Events: %d\n", numEvents)
						fmt.Println()

						for evIndex, ev := range blockEvs {
							prettyPrintEvent("  ", evIndex, ev)
							fmt.Println()
						}
					}
				}

				if len(args) >= 2 {
					fmt.Println()

					// Resolve transaction index if needed.
					if txIndex == -1 {
						for i, tx := range txs {
							if h := tx.Tx.Hash(); h.Equal(&txHash) {
								txIndex = i
								break
							}
						}

						if txIndex == -1 {
							cobra.CheckErr(fmt.Errorf("failed to find transaction with hash %s", txHash))
						}
					}

					if txIndex >= len(txs) {
						cobra.CheckErr(fmt.Errorf("transaction index %d is out of range", txIndex))
					}
					tx := txs[txIndex]

					fmt.Printf("=== Transaction %d ===\n", txIndex)

					if len(tx.Tx.AuthProofs) == 1 && tx.Tx.AuthProofs[0].Module != "" {
						// Module-specific transaction encoding scheme.
						scheme := tx.Tx.AuthProofs[0].Module

						switch scheme {
						case "evm.ethereum.v0":
							// Ethereum transaction encoding.
							var ethTx ethTypes.Transaction
							if err := ethTx.UnmarshalBinary(tx.Tx.Body); err != nil {
								fmt.Printf("[malformed 'evm.ethereum.v0' transaction: %s]\n", err)
								break
							}

							fmt.Printf("Kind:      evm.ethereum.v0\n")
							fmt.Printf("Hash:      %s\n", tx.Tx.Hash())
							fmt.Printf("Eth hash:  %s\n", ethTx.Hash())
							fmt.Printf("Chain ID:  %s\n", ethTx.ChainId())
							fmt.Printf("Nonce:     %d\n", ethTx.Nonce())
							fmt.Printf("Type:      %d\n", ethTx.Type())
							fmt.Printf("To:        %s\n", ethTx.To())
							fmt.Printf("Value:     %s\n", ethTx.Value())
							fmt.Printf("Gas limit: %d\n", ethTx.Gas())
							fmt.Printf("Gas price: %s\n", ethTx.GasPrice())
							fmt.Printf("Data:\n")
							if len(ethTx.Data()) > 0 {
								fmt.Printf("  %s\n", hex.EncodeToString(ethTx.Data()))
							} else {
								fmt.Printf("  (none)\n")
							}
						default:
							fmt.Printf("[module-specific transaction encoding scheme: %s]\n", scheme)
						}
					} else {
						// Regular SDK transaction.
						fmt.Printf("Kind: oasis\n")

						common.PrintTransactionRaw(npa, &tx.Tx)
					}
					fmt.Println()

					// Show result.
					fmt.Printf("=== Result of transaction %d ===\n", txIndex)
					switch res := tx.Result; {
					case res.Failed != nil:
						fmt.Printf("Status:  failed\n")
						fmt.Printf("Module:  %s\n", res.Failed.Module)
						fmt.Printf("Code:    %d\n", res.Failed.Code)
						fmt.Printf("Message: %s\n", res.Failed.Message)
					case res.Ok != nil:
						fmt.Printf("Status: ok\n")
						fmt.Printf("Data:\n")
						prettyPrintCBOR("  ", "result", res.Ok)
					case res.Unknown != nil:
						fmt.Printf("Status: unknown\n")
						fmt.Printf("Data:\n")
						prettyPrintCBOR("  ", "result", res.Unknown)
					default:
						fmt.Printf("[unsupported result kind]\n")
					}
					fmt.Println()

					// Show events.
					fmt.Printf("=== Events emitted by transaction %d ===\n", txIndex)
					if numEvents := len(tx.Events); numEvents > 0 {
						fmt.Printf("Events: %d\n", numEvents)
						fmt.Println()

						for evIndex, ev := range tx.Events {
							prettyPrintEvent("  ", evIndex, ev)
							fmt.Println()
						}
					} else {
						fmt.Println("No events emitted by this transaction.")
					}
				}
			case propertySelector:
				switch v {
				case selParameters:
					showParameters(ctx, npa, selectedRound, rt)
					return
				case selEvents:
					showEvents(ctx, selectedRound, rt)
					return
				default:
					cobra.CheckErr(fmt.Errorf("selector '%s' not found", args[0]))
				}
			}
		},
	}
)

func parseBlockNum(
	s string,
) (interface{}, error) { // TODO: Use `any`
	if sel := selectorFromString(s); sel != selInvalid {
		return sel, nil
	}

	switch blkNumRaw := s; blkNumRaw {
	case selRoundLatest:
		// The latest block.
		return client.RoundLatest, nil // TODO: Support consensus.
	default:
		// A specific block.
		blkNum, err := strconv.ParseUint(blkNumRaw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("malformed block number: %w", err)
		}
		return blkNum, nil
	}
}

func selectorFromString(s string) propertySelector {
	if strings.ToLower(strings.TrimSpace(s)) == "parameters" {
		return selParameters
	}
	if strings.ToLower(strings.TrimSpace(s)) == "events" {
		return selEvents
	}
	return selInvalid
}

func prettyPrintCBOR(indent string, kind string, data []byte) {
	var body interface{}
	if err := cbor.Unmarshal(data, &body); err != nil {
		rawPrintData(indent, kind, data)
		return
	}

	prettyPrintStruct(indent, kind, data, body)
}

func convertPrettyStruct(in interface{}) (interface{}, error) {
	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Invalid:
		return nil, nil
	case reflect.Slice:
		// Walk slices.
		if v.Type().Elem().Kind() == reflect.Uint8 {
			// Encode to hex.
			return hex.EncodeToString(v.Bytes()), nil
		}

		result := v
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			encoded, err := convertPrettyStruct(elem.Interface())
			if err != nil {
				return nil, err
			}
			encodedValue := reflect.ValueOf(encoded)
			if encodedValue.Type() != elem.Type() {
				if result == v {
					// Assumption is that all elements are converted to the same type.
					result = reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(encoded)), v.Len(), v.Cap())
				}
				elem = result.Index(i)
			}
			elem.Set(encodedValue)
		}
		return result.Interface(), nil
	case reflect.Map:
		// Convert maps to map[string]interface{}.
		result := make(map[string]interface{})
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key()
			val := iter.Value()

			keyStr, ok := k.Interface().(string)
			if !ok {
				return nil, fmt.Errorf("can only convert maps with string keys")
			}

			value, err := convertPrettyStruct(val.Interface())
			if err != nil {
				return nil, err
			}
			result[keyStr] = value
		}
		return result, nil
	case reflect.Struct:
		// Convert structs to map[string]interface{}.
		result := make(map[string]interface{})
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" {
				// Skip unexported fields.
				continue
			}

			key := field.Name
			if tagValue := field.Tag.Get("json"); tagValue != "" {
				attrs := strings.Split(tagValue, ",")
				key = attrs[0]
			}

			value, err := convertPrettyStruct(v.Field(i).Interface())
			if err != nil {
				return nil, err
			}
			result[key] = value
		}
		return result, nil
	default:
		// Pass everything else unchanged.
		return v.Interface(), nil
	}
}

func rawPrintData(indent string, kind string, data []byte) {
	fmt.Printf("%s[showing raw %s data]\n", indent, kind)
	fmt.Printf("%s%s\n", indent, hex.EncodeToString(data))
}

func prettyPrintStruct(indent string, kind string, data []byte, body interface{}) {
	// TODO: Support the pretty printer.
	body, err := convertPrettyStruct(body)
	if err != nil {
		rawPrintData(indent, kind, data)
		return
	}

	pretty, err := json.MarshalIndent(body, indent, "  ")
	if err != nil {
		rawPrintData(indent, kind, data)
		return
	}

	output := string(pretty)
	fmt.Println(indent + output)
}

func prettyPrintEvent(indent string, evIndex int, ev *types.Event) {
	fmt.Printf("%s--- Event %d ---\n", indent, evIndex)
	fmt.Printf("%sModule: %s\n", indent, ev.Module)
	fmt.Printf("%sCode:   %d\n", indent, ev.Code)
	if ev.TxHash != nil {
		fmt.Printf("%sTx hash: %s\n", indent, ev.TxHash.String())
	}
	fmt.Printf("%sData:\n", indent)

	for _, decoder := range eventDecoders {
		decoded, err := decoder(ev)
		if err != nil {
			continue
		}
		if decoded != nil {
			prettyPrintStruct(indent+"  ", "event", ev.Value, decoded)
			return
		}
	}

	prettyPrintCBOR(indent+"  ", "event", ev.Value)
}

func jsonPrintEvents(evs []*types.Event) {
	out := []map[string]interface{}{}

	for _, ev := range evs {
		fields := make(map[string]interface{})
		fields["module"] = ev.Module
		fields["code"] = ev.Code
		if ev.TxHash != nil {
			fields["tx_hash"] = ev.TxHash.String()
		}
		fields["data"] = ev.Value

		for _, decoder := range eventDecoders {
			decoded, err := decoder(ev)
			if err != nil {
				continue
			}
			if decoded != nil {
				fields["parsed"] = decoded

				break
			}
		}
		out = append(out, fields)
	}

	str, err := common.PrettyJSONMarshal(out)
	cobra.CheckErr(err)
	fmt.Printf("%s\n", str)
}

func showParameters(ctx context.Context, npa *common.NPASelection, round uint64, rt connection.RuntimeClient) {
	checkErr := func(what string, err error) {
		if err != nil {
			cobra.CheckErr(fmt.Errorf("%s: %w", what, err))
		}
	}

	roflStakeThresholds, err := rt.ROFL.StakeThresholds(ctx, round)
	checkErr("ROFL StakeThresholds", err)

	roflMarketStakeThresholds, err := rt.ROFLMarket.StakeThresholds(ctx, round)
	checkErr("ROFL Market StakeThresholds", err)

	doc := make(map[string]interface{})

	doSection := func(name string, params interface{}) {
		if common.OutputFormat() == common.FormatJSON {
			doc[name] = params
		} else {
			fmt.Printf("\n=== %s PARAMETERS ===\n", strings.ToUpper(name))
			out := common.PrettyPrint(npa, "  ", params)
			fmt.Printf("%s\n", out)
		}
	}

	doSection("rofl", roflStakeThresholds)
	doSection("rofl market", roflMarketStakeThresholds)

	if common.OutputFormat() == common.FormatJSON {
		pp, err := json.MarshalIndent(doc, "", "  ")
		cobra.CheckErr(err)
		fmt.Printf("%s\n", pp)
	}
}

func showEvents(ctx context.Context, round uint64, rt connection.RuntimeClient) {
	evs, err := rt.GetEventsRaw(ctx, round)
	cobra.CheckErr(err)

	if len(evs) == 0 {
		if common.OutputFormat() == common.FormatJSON {
			fmt.Printf("[]\n")
		} else {
			fmt.Println("No events emitted in this block.")
		}
		return
	}

	if common.OutputFormat() == common.FormatJSON {
		jsonPrintEvents(evs)
	} else {
		for evIndex, ev := range evs {
			prettyPrintEvent("", evIndex, ev)
			fmt.Println()
		}
	}
}

func init() {
	roundFlag := flag.NewFlagSet("", flag.ContinueOnError)
	roundFlag.Uint64Var(&selectedRound, "round", client.RoundLatest, "explicitly set block round to use")

	showCmd.Flags().AddFlagSet(common.FormatFlag)
	showCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	showCmd.Flags().AddFlagSet(roundFlag)
}
