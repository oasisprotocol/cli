package show

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

type accountShowOutput struct {
	Name                         string                                                 `json:"name,omitempty"`
	EthereumAddress              *ethCommon.Address                                     `json:"ethereum_address,omitempty"`
	NativeAddress                *types.Address                                         `json:"native_address"`
	Nonce                        uint64                                                 `json:"nonce"`
	NetworkName                  string                                                 `json:"network_name"`
	Height                       int64                                                  `json:"height"`
	GeneralAccount               *staking.GeneralAccount                                `json:"general_account,omitempty"`
	OutgoingDelegations          map[staking.Address]*staking.DelegationInfo            `json:"outgoing_delegations,omitempty"`
	OutgoingDebondingDelegations map[staking.Address][]*staking.DebondingDelegationInfo `json:"outgoing_debonding_delegations,omitempty"`
	IncomingDelegations          map[staking.Address]*staking.Delegation                `json:"incoming_delegations,omitempty"`
	IncomingDebondingDelegations map[staking.Address][]*staking.DebondingDelegation     `json:"incoming_debonding_delegations,omitempty"`
	EscrowAccount                *staking.EscrowAccount                                 `json:"escrow_account,omitempty"`
	ParaTimeName                 string                                                 `json:"paratime_name,omitempty"`
	ParaTimeBalances             map[types.Denomination]types.Quantity                  `json:"paratime_balances,omitempty"`
	ParaTimeNonce                uint64                                                 `json:"paratime_nonce,omitempty"`
	ParaTimeDelegations          []*consensusaccounts.ExtendedDelegationInfo            `json:"paratime_delegations,omitempty"`
	ParaTimeUndelegations        []*consensusaccounts.UndelegationInfo                  `json:"paratime_undelegations,omitempty"`
}

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

			var out accountShowOutput
			out.NetworkName = npa.NetworkName

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
			out.EthereumAddress = ethAddr
			out.NativeAddress = nativeAddr
			out.Name = common.FindAccountName(nativeAddr.String())

			height, err := common.GetActualHeight(
				ctx,
				c.Consensus().Core(),
			)
			cobra.CheckErr(err)
			out.Height = height

			ownerQuery := &staking.OwnerQuery{
				Owner:  nativeAddr.ConsensusAddress(),
				Height: height,
			}

			// Query consensus layer account.
			consensusAccount, err := c.Consensus().Staking().Account(ctx, ownerQuery)
			cobra.CheckErr(err)
			out.EscrowAccount = &consensusAccount.Escrow
			out.GeneralAccount = &consensusAccount.General
			out.Nonce = consensusAccount.General.Nonce

			if showDelegations {
				out.OutgoingDelegations, err = c.Consensus().Staking().DelegationInfosFor(ctx, ownerQuery)
				cobra.CheckErr(err)
				out.OutgoingDebondingDelegations, err = c.Consensus().Staking().DebondingDelegationInfosFor(ctx, ownerQuery)
				cobra.CheckErr(err)
				out.IncomingDelegations, err = c.Consensus().Staking().DelegationsTo(ctx, ownerQuery)
				cobra.CheckErr(err)
				out.IncomingDebondingDelegations, err = c.Consensus().Staking().DebondingDelegationsTo(ctx, ownerQuery)
				cobra.CheckErr(err)
			}

			if npa.ParaTime != nil {
				out.ParaTimeName = npa.ParaTimeName

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
				out.ParaTimeBalances = rtBalances.Balances

				out.ParaTimeNonce, err = c.Runtime(npa.ParaTime).Accounts.Nonce(ctx, round, *nativeAddr)
				cobra.CheckErr(err)

				if showDelegations {
					out.ParaTimeDelegations, err = c.Runtime(npa.ParaTime).ConsensusAccounts.Delegations(
						ctx,
						round,
						&consensusaccounts.DelegationsQuery{
							From: *nativeAddr,
						},
					)
					cobra.CheckErr(err)
					out.ParaTimeUndelegations, err = c.Runtime(npa.ParaTime).ConsensusAccounts.Undelegations(
						ctx,
						round,
						&consensusaccounts.UndelegationsQuery{
							To: *nativeAddr,
						},
					)
					cobra.CheckErr(err)
				}
			}

			if common.OutputFormat() == common.FormatJSON {
				data, err := json.MarshalIndent(out, "", "  ")
				cobra.CheckErr(err)
				fmt.Printf("%s\n", data)
			} else {
				prettyPrintAccount(ctx, c, npa.Network, npa.ParaTime, &out)
			}
		},
	}
)

// prettyPrintAccount prints a compact human-readable summary of the account.
func prettyPrintAccount(ctx context.Context, c connection.Connection, network *config.Network, pt *config.ParaTime, out *accountShowOutput) {
	if out.Name != "" {
		fmt.Printf("Name:             %s\n", out.Name)
	}
	if out.EthereumAddress != nil {
		fmt.Printf("Ethereum address: %s\n", out.EthereumAddress)
	}
	fmt.Printf("Native address:   %s\n", out.NativeAddress)
	fmt.Println()
	fmt.Printf("=== CONSENSUS LAYER (%s) ===\n", out.NetworkName)
	fmt.Printf("  Nonce: %d\n", out.Nonce)
	fmt.Println()

	prettyPrintAccountBalanceAndDelegationsFrom(
		network,
		out.NativeAddress,
		*out.GeneralAccount,
		out.OutgoingDelegations,
		out.OutgoingDebondingDelegations,
		"  ",
		os.Stdout,
	)

	if len(out.GeneralAccount.Allowances) > 0 {
		fmt.Println("  Allowances for this Account:")
		prettyPrintAllowances(
			network,
			out.NativeAddress,
			out.GeneralAccount.Allowances,
			"    ",
			os.Stdout,
		)
		fmt.Println()
	}

	if len(out.IncomingDelegations) > 0 {
		fmt.Println("  Active Delegations to this Account:")
		prettyPrintDelegationsTo(
			network,
			out.NativeAddress,
			out.EscrowAccount.Active,
			out.IncomingDelegations,
			"    ",
			os.Stdout,
		)
		fmt.Println()
	}

	if len(out.IncomingDebondingDelegations) > 0 {
		fmt.Println("  Debonding Delegations to this Account:")
		prettyPrintDelegationsTo(
			network,
			out.NativeAddress,
			out.EscrowAccount.Debonding,
			out.IncomingDebondingDelegations,
			"    ",
			os.Stdout,
		)
		fmt.Println()
	}

	if ea := out.EscrowAccount; ea != nil {
		if len(ea.CommissionSchedule.Rates) > 0 || len(ea.CommissionSchedule.Bounds) > 0 {
			fmt.Println("  Commission Schedule:")
			ea.CommissionSchedule.PrettyPrint(ctx, "    ", os.Stdout)
			fmt.Println()
		}
		if len(ea.StakeAccumulator.Claims) > 0 {
			fmt.Println("  Stake Accumulator:")
			ea.StakeAccumulator.PrettyPrint(ctx, "    ", os.Stdout)
			fmt.Println()
		}
	}

	if out.ParaTimeNonce > 0 || len(out.ParaTimeBalances) > 0 || len(out.ParaTimeDelegations) > 0 || len(out.ParaTimeUndelegations) > 0 {
		fmt.Printf("=== %s PARATIME ===\n", out.ParaTimeName)
		fmt.Printf("  Nonce: %d\n", out.ParaTimeNonce)
		fmt.Println()

		if balances := out.ParaTimeBalances; balances != nil {
			fmt.Printf("  Balances for all denominations:\n")
			for denom, balance := range balances {
				fmtAmnt := helpers.FormatParaTimeDenomination(pt, types.NewBaseUnits(balance, denom))
				amnt, symbol, _ := strings.Cut(fmtAmnt, " ")

				fmt.Printf("  - Amount: %s\n", amnt)
				fmt.Printf("    Symbol: %s\n", symbol)
			}

			fmt.Println()

			if showDelegations {
				prettyPrintParaTimeDelegations(
					ctx,
					c,
					out.Height,
					network,
					out.NativeAddress,
					out.ParaTimeDelegations,
					out.ParaTimeUndelegations,
					"  ",
					os.Stdout,
				)
			}
		}
	}
}

func init() {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.BoolVar(&showDelegations, "show-delegations", false, "show incoming and outgoing delegations")
	common.AddSelectorFlags(Cmd)
	Cmd.Flags().AddFlagSet(common.HeightFlag)
	Cmd.Flags().AddFlagSet(common.FormatFlag)
	Cmd.Flags().AddFlagSet(f)
}
