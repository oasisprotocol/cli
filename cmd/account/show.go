package account

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
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

	showCmd = &cobra.Command{
		Use:     "show [address]",
		Short:   "Show balance and other information",
		Aliases: []string{"s"},
		Args:    cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
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

			helpers.PrettyPrintAccountBalanceAndDelegationsFrom(
				npa.Network,
				addr,
				consensusAccount.General,
				outgoingDelegations,
				outgoingDebondingDelegations,
				"  ",
				os.Stdout,
			)
			fmt.Println()

			if len(consensusAccount.General.Allowances) > 0 {
				fmt.Println("  Allowances for this Account:")
				helpers.PrettyPrintAllowances(
					npa.Network,
					addr,
					consensusAccount.General.Allowances,
					"    ",
					os.Stdout,
				)
			}

			if showDelegations {
				incomingDelegations, err := c.Consensus().Staking().DelegationsTo(ctx, ownerQuery)
				cobra.CheckErr(err)
				incomingDebondingDelegations, err := c.Consensus().Staking().DebondingDelegationsTo(ctx, ownerQuery)
				cobra.CheckErr(err)

				if len(incomingDelegations) > 0 {
					fmt.Println("  Active Delegations to this Account:")
					helpers.PrettyPrintDelegationsTo(
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
					helpers.PrettyPrintDelegationsTo(
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
					fmt.Println()
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
						rtDelegations, err := c.Runtime(npa.ParaTime).ConsensusAccounts.Delegations(
							ctx,
							round,
							&consensusaccounts.DelegationsQuery{
								From: *addr,
							},
						)
						if err == nil && len(rtDelegations) > 0 {
							showParaTimeDelegations(ctx, c, height, npa, rtDelegations)
							fmt.Println()
						}
					}
				}
			}
		},
	}
)

func showParaTimeDelegations(
	ctx context.Context,
	c connection.Connection,
	height int64,
	npa *common.NPASelection,
	rtDelegations []*consensusaccounts.ExtendedDelegationInfo,
) {
	type extendedDelegationInfo struct {
		to     types.Address
		amount quantity.Quantity
		shares quantity.Quantity
	}

	var (
		total       quantity.Quantity
		delegations []extendedDelegationInfo
	)
	for _, di := range rtDelegations {
		// For each destination we need to fetch the pool.
		destAccount, err := c.Consensus().Staking().Account(ctx, &staking.OwnerQuery{
			Owner:  di.To.ConsensusAddress(),
			Height: height,
		})
		cobra.CheckErr(err)

		// Then we can compute the current amount.
		amount, _ := destAccount.Escrow.Active.StakeForShares(&di.Shares)
		_ = total.Add(amount)

		delegations = append(delegations, extendedDelegationInfo{
			to:     di.To,
			amount: *amount,
			shares: di.Shares,
		})
	}

	fmt.Printf("  Active delegations from this Account:\n")
	fmt.Printf("    Total: %s\n", helpers.FormatConsensusDenomination(npa.Network, total))
	fmt.Println()

	fmt.Printf("    Delegations:\n")
	for _, di := range delegations {
		fmt.Printf("    - To:     %s\n", di.to)
		fmt.Printf("      Amount: %s (%s shares)\n", helpers.FormatConsensusDenomination(npa.Network, di.amount), di.shares)
	}
}

func init() {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.BoolVar(&showDelegations, "show-delegations", false, "show incoming and outgoing delegations")
	showCmd.Flags().AddFlagSet(common.SelectorFlags)
	showCmd.Flags().AddFlagSet(common.HeightFlag)
	showCmd.Flags().AddFlagSet(f)
}
