package inspect

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

const (
	blockLatest = "latest"
)

var blockCmd = &cobra.Command{
	Use:   "block <number> [ <tx-index> | <tx-hash> ]",
	Short: "Show information about a block and its transactions",
	Long:  "Show information about a given block number and (optionally) its transactions. Use \"latest\" to use the last block.",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)

		var (
			err     error
			blkNum  uint64
			txIndex int
			txHash  hash.Hash
		)
		switch blkNumRaw := args[0]; blkNumRaw {
		case blockLatest:
			// The latest block.
			blkNum = client.RoundLatest // TODO: Support consensus.
		default:
			// A specific block.
			blkNum, err = strconv.ParseUint(blkNumRaw, 10, 64)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("malformed block number: %w", err))
			}
		}

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

		fmt.Printf("Network:        %s", npa.NetworkName)
		if len(npa.Network.Description) > 0 {
			fmt.Printf(" (%s)", npa.Network.Description)
		}
		fmt.Println()

		switch npa.ParaTime {
		case nil:
			// Consensus layer.
			cobra.CheckErr("inspecting consensus layer blocks not yet supported") // TODO
		default:
			// Runtime layer.
			rt := conn.Runtime(npa.ParaTime)

			evDecoders := []client.EventDecoder{
				rt.Accounts,
				rt.ConsensusAccounts,
				rt.Contracts,
				rt.Evm,
			}

			blk, err := rt.GetBlock(ctx, blkNum)
			cobra.CheckErr(err)

			fmt.Printf("ParaTime:       %s", npa.ParaTimeName)
			if len(npa.ParaTime.Description) > 0 {
				fmt.Printf(" (%s)", npa.ParaTime.Description)
			}
			fmt.Println()
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
							fmt.Printf("  %s\n", base64.StdEncoding.EncodeToString(ethTx.Data()))
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
					prettyPrintCBOR("  ", res.Ok)
				case res.Unknown != nil:
					fmt.Printf("Status: unknown\n")
					fmt.Printf("Data:\n")
					prettyPrintCBOR("  ", res.Unknown)
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
						prettyPrintEvent("  ", evIndex, ev, evDecoders)
						fmt.Println()
					}
				} else {
					fmt.Println("No events emitted by this transaction.")
				}
			}
		}
	},
}

func prettyPrintCBOR(indent string, data []byte) {
	var body interface{}
	if err := cbor.Unmarshal(data, &body); err != nil {
		fmt.Printf("%s%s\n", indent, base64.StdEncoding.EncodeToString(data))
		return
	}

	prettyPrintStruct(indent, data, body)
}

func prettyPrintStruct(indent string, data []byte, body interface{}) {
	// TODO: Support the pretty printer.
	pretty, err := yaml.Marshal(body)
	if err != nil {
		fmt.Printf("%s%s\n", indent, base64.StdEncoding.EncodeToString(data))
		return
	}

	output := string(pretty)
	output = strings.ReplaceAll(output, "\n", "\n"+indent)
	output = strings.TrimSpace(output)
	fmt.Println(indent + output)
}

func prettyPrintEvent(indent string, evIndex int, ev *types.Event, decoders []client.EventDecoder) {
	fmt.Printf("%s--- Event %d ---\n", indent, evIndex)
	fmt.Printf("%sModule: %s\n", indent, ev.Module)
	fmt.Printf("%sCode:   %d\n", indent, ev.Code)
	fmt.Printf("%sData:\n", indent)

	for _, decoder := range decoders {
		decoded, err := decoder.DecodeEvent(ev)
		if err != nil {
			continue
		}
		if decoded != nil {
			prettyPrintStruct(indent+"  ", ev.Value, decoded)
			return
		}
	}

	prettyPrintCBOR(indent+"  ", ev.Value)
}
