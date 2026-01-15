package show

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	showDelegations bool

	Cmd = &cobra.Command{
		Use:               "show [address]",
		Short:             "Show balance and other information",
		Aliases:           []string{"s", "balance", "b"},
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: common.CompleteAccountAndAddressBookNames,
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			// Determine which address to show. If an explicit argument was given, use that
			// otherwise use the default account.
			var targetAddress string
			switch {
			case len(args) >= 1:
				// Explicit argument given.
				targetAddress = args[0]
			case npa.Account != nil:
				// Default account is selected.
				targetAddress = npa.Account.Address
			default:
				// No address given and no wallet configured.
				cobra.CheckErr("no address given and no wallet configured")
			}

			// Establish connection with the target network.
			ctx := context.Background()
			c, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			nativeAddr, ethAddr, err := common.ResolveLocalAccountOrAddress(npa.Network, targetAddress)
			cobra.CheckErr(err)

			jsonOut := map[string]interface{}{}
			if name := common.FindAccountName(nativeAddr.String()); name != "" {
				if common.OutputFormat() == common.FormatJSON {
					jsonOut["name"] = name
				} else {
					fmt.Printf("Name:             %s\n", name)
				}
			}

			height, err := common.GetActualHeight(
				ctx,
				c.Consensus().Core(),
			)
			cobra.CheckErr(err)

			ownerQuery := &staking.OwnerQuery{
				Owner:  nativeAddr.ConsensusAddress(),
				Height: height,
			}

			// Query consensus layer account.
			// TODO: Nicer overall formatting.

			consensusAccount, err := c.Consensus().Staking().Account(ctx, ownerQuery)
			cobra.CheckErr(err)

			if common.OutputFormat() == common.FormatJSON {
				jsonOut["ethereum_address"] = ethAddr
				jsonOut["native_address"] = nativeAddr
				jsonOut["nonce"] = consensusAccount.General.Nonce
				jsonOut["network_name"] = npa.NetworkName
			} else {
				if ethAddr != nil {
					fmt.Printf("Ethereum address: %s\n", ethAddr)
				}
				fmt.Printf("Native address:   %s\n", nativeAddr)
				fmt.Println()
				fmt.Printf("=== CONSENSUS LAYER (%s) ===\n", npa.NetworkName)
				fmt.Printf("  Nonce: %d\n", consensusAccount.General.Nonce)
				fmt.Println()
			}
			var (
				outgoingDelegations          map[staking.Address]*staking.DelegationInfo
				outgoingDebondingDelegations map[staking.Address][]*staking.DebondingDelegationInfo
			)
			if showDelegations {
				outgoingDelegations, err = c.Consensus().Staking().DelegationInfosFor(ctx, ownerQuery)
				cobra.CheckErr(err)
				outgoingDebondingDelegations, err = c.Consensus().Staking().DebondingDelegationInfosFor(ctx, ownerQuery)
				cobra.CheckErr(err)
			}

			if common.OutputFormat() == common.FormatJSON {
				jsonOut["general"] = &consensusAccount.General
				jsonOut["outgoing_delegations"] = outgoingDelegations
				jsonOut["outgoing_debonding_delegations"] = outgoingDebondingDelegations
			} else {
				prettyPrintAccountBalanceAndDelegationsFrom(
					npa.Network,
					nativeAddr,
					consensusAccount.General,
					outgoingDelegations,
					outgoingDebondingDelegations,
					"  ",
					os.Stdout,
				)
			}

			if len(consensusAccount.General.Allowances) > 0 && common.OutputFormat() == common.FormatText {
				fmt.Println("  Allowances for this Account:")
				prettyPrintAllowances(
					npa.Network,
					nativeAddr,
					consensusAccount.General.Allowances,
					"    ",
					os.Stdout,
				)
				fmt.Println()
			}

			if showDelegations {
				incomingDelegations, err := c.Consensus().Staking().DelegationsTo(ctx, ownerQuery)
				cobra.CheckErr(err)
				incomingDebondingDelegations, err := c.Consensus().Staking().DebondingDelegationsTo(ctx, ownerQuery)
				cobra.CheckErr(err)

				if len(incomingDelegations) > 0 {
					if common.OutputFormat() == common.FormatJSON {
						jsonOut["incoming_delegations"] = incomingDelegations
					} else {
						fmt.Println("  Active Delegations to this Account:")
						prettyPrintDelegationsTo(
							npa.Network,
							nativeAddr,
							consensusAccount.Escrow.Active,
							incomingDelegations,
							"    ",
							os.Stdout,
						)
						fmt.Println()
					}
				}
				if len(incomingDebondingDelegations) > 0 {
					if common.OutputFormat() == common.FormatJSON {
						jsonOut["incoming_debonding_delegations"] = incomingDebondingDelegations
					} else {
						fmt.Println("  Debonding Delegations to this Account:")
						prettyPrintDelegationsTo(
							npa.Network,
							nativeAddr,
							consensusAccount.Escrow.Debonding,
							incomingDebondingDelegations,
							"    ",
							os.Stdout,
						)
						fmt.Println()
					}
				}
			}

			cs := consensusAccount.Escrow.CommissionSchedule
			if (len(cs.Rates) > 0 || len(cs.Bounds) > 0) && common.OutputFormat() == common.FormatText {
				fmt.Println("  Commission Schedule:")
				cs.PrettyPrint(ctx, "    ", os.Stdout)
				fmt.Println()
			}

			sa := consensusAccount.Escrow.StakeAccumulator
			if len(sa.Claims) > 0 && common.OutputFormat() == common.FormatText {
				fmt.Println("  Stake Accumulator:")
				sa.PrettyPrint(ctx, "    ", os.Stdout)
				fmt.Println()
			}

			if npa.ParaTime != nil {
				// Make an effort to support the height query.
				//
				// Note: Public gRPC endpoints do not allow this method.
				round := client.RoundLatest
				if h := common.GetHeight(); h != consensus.HeightLatest {
					blk, err := c.Consensus().RootHash().GetLatestBlock(
						ctx,
						&roothash.RuntimeRequest{
							RuntimeID: npa.ParaTime.Namespace(),
							Height:    height,
						},
					)
					cobra.CheckErr(err)
					round = blk.Header.Round
				}

				// Query runtime account when a ParaTime has been configured.
				rtBalances, err := c.Runtime(npa.ParaTime).Accounts.Balances(ctx, round, *nativeAddr)
				cobra.CheckErr(err)

				var hasNonZeroBalance bool
				for _, balance := range rtBalances.Balances {
					if hasNonZeroBalance = !balance.IsZero(); hasNonZeroBalance {
						break
					}
				}

				nonce, err := c.Runtime(npa.ParaTime).Accounts.Nonce(ctx, round, *nativeAddr)
				cobra.CheckErr(err)
				hasNonZeroNonce := nonce > 0

				if hasNonZeroBalance || hasNonZeroNonce {
					if common.OutputFormat() == common.FormatJSON {
						jsonOut["paratime_name"] = npa.ParaTimeName
						jsonOut["paratime_nonce"] = nonce
					} else {
						fmt.Printf("=== %s PARATIME ===\n", npa.ParaTimeName)
						fmt.Printf("  Nonce: %d\n", nonce)
						fmt.Println()
					}

					if hasNonZeroBalance {
						if common.OutputFormat() == common.FormatJSON {
							jsonOut["paratime_balances"] = rtBalances.Balances
						} else {
							fmt.Printf("  Balances for all denominations:\n")
							for denom, balance := range rtBalances.Balances {
								fmtAmnt := helpers.FormatParaTimeDenomination(npa.ParaTime, types.NewBaseUnits(balance, denom))
								amnt, symbol, _ := strings.Cut(fmtAmnt, " ")

								fmt.Printf("  - Amount: %s\n", amnt)
								fmt.Printf("    Symbol: %s\n", symbol)
							}

							fmt.Println()
						}
					}

					if showDelegations {
						rtDelegations, _ := c.Runtime(npa.ParaTime).ConsensusAccounts.Delegations(
							ctx,
							round,
							&consensusaccounts.DelegationsQuery{
								From: *nativeAddr,
							},
						)
						rtUndelegations, _ := c.Runtime(npa.ParaTime).ConsensusAccounts.Undelegations(
							ctx,
							round,
							&consensusaccounts.UndelegationsQuery{
								To: *nativeAddr,
							},
						)
						if common.OutputFormat() == common.FormatJSON {
							jsonOut["paratime_delegations"] = rtDelegations
							jsonOut["paratime_undelegations"] = rtUndelegations
						} else {
							prettyPrintParaTimeDelegations(ctx, c, height, npa, nativeAddr, rtDelegations, rtUndelegations, "  ", os.Stdout)
						}
					}
				}
			}

			if common.OutputFormat() == common.FormatJSON {
				data, err := json.MarshalIndent(jsonOut, "", "  ")
				cobra.CheckErr(err)
				fmt.Printf("%s\n", data)
			}
		},
	}
)

func init() {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.BoolVar(&showDelegations, "show-delegations", false, "show incoming and outgoing delegations")
	common.AddSelectorFlags(Cmd)
	Cmd.Flags().AddFlagSet(common.HeightFlag)
	Cmd.Flags().AddFlagSet(common.FormatFlag)
	Cmd.Flags().AddFlagSet(f)
}
