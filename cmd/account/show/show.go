package show

import (
	"context"
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
		Use:     "show [address]",
		Short:   "Show balance and other information",
		Aliases: []string{"s"},
		Args:    cobra.MaximumNArgs(1),
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

			addr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, targetAddress)
			cobra.CheckErr(err)

			height, err := common.GetActualHeight(
				ctx,
				c.Consensus(),
			)
			cobra.CheckErr(err)

			ownerQuery := &staking.OwnerQuery{
				Owner:  addr.ConsensusAddress(),
				Height: height,
			}

			// Query consensus layer account.
			// TODO: Nicer overall formatting.

			consensusAccount, err := c.Consensus().Staking().Account(ctx, ownerQuery)
			cobra.CheckErr(err)

			fmt.Printf("Address: %s\n", addr)
			fmt.Println()
			fmt.Printf("=== CONSENSUS LAYER (%s) ===\n", npa.NetworkName)
			fmt.Printf("  Nonce: %d\n", consensusAccount.General.Nonce)
			fmt.Println()

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

			prettyPrintAccountBalanceAndDelegationsFrom(
				npa.Network,
				addr,
				consensusAccount.General,
				outgoingDelegations,
				outgoingDebondingDelegations,
				"  ",
				os.Stdout,
			)

			if len(consensusAccount.General.Allowances) > 0 {
				fmt.Println("  Allowances for this Account:")
				prettyPrintAllowances(
					npa.Network,
					addr,
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
					fmt.Println("  Active Delegations to this Account:")
					prettyPrintDelegationsTo(
						npa.Network,
						addr,
						consensusAccount.Escrow.Active,
						incomingDelegations,
						"    ",
						os.Stdout,
					)
					fmt.Println()
				}
				if len(incomingDebondingDelegations) > 0 {
					fmt.Println("  Debonding Delegations to this Account:")
					prettyPrintDelegationsTo(
						npa.Network,
						addr,
						consensusAccount.Escrow.Debonding,
						incomingDebondingDelegations,
						"    ",
						os.Stdout,
					)
					fmt.Println()
				}
			}

			cs := consensusAccount.Escrow.CommissionSchedule
			if len(cs.Rates) > 0 || len(cs.Bounds) > 0 {
				fmt.Println("  Commission Schedule:")
				cs.PrettyPrint(ctx, "    ", os.Stdout)
				fmt.Println()
			}

			sa := consensusAccount.Escrow.StakeAccumulator
			if len(sa.Claims) > 0 {
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
				rtBalances, err := c.Runtime(npa.ParaTime).Accounts.Balances(ctx, round, *addr)
				cobra.CheckErr(err)

				var hasNonZeroBalance bool
				for _, balance := range rtBalances.Balances {
					if hasNonZeroBalance = !balance.IsZero(); hasNonZeroBalance {
						break
					}
				}

				nonce, err := c.Runtime(npa.ParaTime).Accounts.Nonce(ctx, round, *addr)
				cobra.CheckErr(err)
				hasNonZeroNonce := nonce > 0

				if hasNonZeroBalance || hasNonZeroNonce {
					fmt.Printf("=== %s PARATIME ===\n", npa.ParaTimeName)
					fmt.Printf("  Nonce: %d\n", nonce)
					fmt.Println()

					if hasNonZeroBalance {
						fmt.Printf("  Balances for all denominations:\n")
						for denom, balance := range rtBalances.Balances {
							fmtAmnt := helpers.FormatParaTimeDenomination(npa.ParaTime, types.NewBaseUnits(balance, denom))
							amnt, symbol, _ := strings.Cut(fmtAmnt, " ")

							fmt.Printf("  - Amount: %s\n", amnt)
							fmt.Printf("    Symbol: %s\n", symbol)
						}

						fmt.Println()
					}

					if showDelegations {
						rtDelegations, _ := c.Runtime(npa.ParaTime).ConsensusAccounts.Delegations(
							ctx,
							round,
							&consensusaccounts.DelegationsQuery{
								From: *addr,
							},
						)
						rtUndelegations, _ := c.Runtime(npa.ParaTime).ConsensusAccounts.Undelegations(
							ctx,
							round,
							&consensusaccounts.UndelegationsQuery{
								To: *addr,
							},
						)
						prettyPrintParaTimeDelegations(ctx, c, height, npa, addr, rtDelegations, rtUndelegations, "  ", os.Stdout)
					}
				}
			}
		},
	}
)

func init() {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.BoolVar(&showDelegations, "show-delegations", false, "show incoming and outgoing delegations")
	Cmd.Flags().AddFlagSet(common.SelectorFlags)
	Cmd.Flags().AddFlagSet(common.HeightFlag)
	Cmd.Flags().AddFlagSet(f)
}
