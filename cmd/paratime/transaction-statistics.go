package paratime

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/oasisprotocol/oasis-core/go/common/prettyprint"
	"github.com/oasisprotocol/oasis-core/go/roothash/api/block"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	configSdk "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	fromStr     string
	toStr       string
	latestBlock *block.Block
)

type Filter struct {
	Name           string
	fromEthAddr    *ethCommon.Address
	fromNativeAddr *types.Address
	toEthAddr      *ethCommon.Address
	toNativeAddr   *types.Address
}

func newFilter(filter string, net *configSdk.Network) (*Filter, error) {
	if !strings.Contains(filter, "->") {
		return nil, fmt.Errorf("invalid filter format '%s'. Should be formatted [from]->[to]", filter)
	}

	addrStr := strings.Split(filter, "->")
	fromAddrStr, toAddrStr := addrStr[0], addrStr[1]

	fromNativeAddr, fromEthAddr, err := common.ResolveLocalAccountOrAddress(net, fromAddrStr)
	if fromAddrStr != "" && err != nil {
		return nil, err
	}
	toNativeAddr, toEthAddr, err := common.ResolveLocalAccountOrAddress(net, toAddrStr)
	if toAddrStr != "" && err != nil {
		return nil, err
	}

	return &Filter{
		Name:           filter,
		fromEthAddr:    fromEthAddr,
		fromNativeAddr: fromNativeAddr,
		toEthAddr:      toEthAddr,
		toNativeAddr:   toNativeAddr,
	}, nil
}

func (r *Filter) Match(from *types.Address, to *types.Address) bool {
	switch r.fromNativeAddr {
	case nil:
	default:
		if from == nil || !r.fromNativeAddr.Equal(*from) {
			return false
		}
	}

	switch r.toNativeAddr {
	case nil:
	default:
		if to == nil || !r.toNativeAddr.Equal(*to) {
			return false
		}
	}

	return true
}

type txStats struct {
	// Date -> Filter -> Number of daily txes.
	Filters   []*Filter
	Dates     []string
	DailyTxes [][]int

	// Date -> Denomination -> Filter -> Amount of daily transferred tokens.
	// The first element of the filter dimension are the totals over all
	// transactions.
	DailyAmounts []map[types.Denomination][]types.Quantity
	// Denominations encountered while scraping the blocks.
	Denominations []types.Denomination

	paratime *configSdk.ParaTime
}

func newTxStats(filters []*Filter, paratime *configSdk.ParaTime) *txStats {
	return &txStats{
		Filters:   filters,
		Dates:     nil,
		DailyTxes: nil,
		paratime:  paratime,
	}
}

// addAmount adds the given amount to the daily total of the filter with the
// given index, where index 0 are the totals over all transactions.
func (t *txStats) addAmount(filterIdx int, amount types.BaseUnits) {
	amounts := t.DailyAmounts[len(t.DailyAmounts)-1]
	if _, ok := amounts[amount.Denomination]; !ok {
		amounts[amount.Denomination] = make([]types.Quantity, len(t.Filters)+1)
		if !slices.Contains(t.Denominations, amount.Denomination) {
			t.Denominations = append(t.Denominations, amount.Denomination)
		}
	}
	if err := amounts[amount.Denomination][filterIdx].Add(&amount.Amount); err != nil {
		fmt.Fprintf(os.Stderr, "warning: unable to add amount %s: %v\n", amount.String(), err)
	}
}

func (t *txStats) AnalyzeRuntimeBlocks(roundFrom, roundTo uint64, rt connection.RuntimeClient) error {
	ctx := context.Background()
	round := roundFrom
	for round <= roundTo {
		blk, err := rt.GetBlock(ctx, round)
		if err != nil {
			return err
		}
		blkDate := time.Unix(int64(blk.Header.Timestamp), 0).Format(time.DateOnly)
		if len(t.Dates) == 0 || t.Dates[len(t.Dates)-1] != blkDate {
			t.Dates = append(t.Dates, blkDate)
			t.DailyTxes = append(t.DailyTxes, make([]int, len(t.Filters)+1))
			t.DailyAmounts = append(t.DailyAmounts, map[types.Denomination][]types.Quantity{})
		}
		txes, err := rt.GetTransactionsWithResults(ctx, round)
		if err != nil {
			return err
		}

		for _, txr := range txes {
			tx := txr.Tx
			var txFrom, txTo *types.Address
			if len(tx.AuthProofs) == 1 && tx.AuthProofs[0].Module != "" {
				// Module-specific transaction encoding scheme.
				scheme := tx.AuthProofs[0].Module

				switch scheme {
				case "evm.ethereum.v0":
					// Ethereum transaction encoding.
					var ethTx ethTypes.Transaction
					if err := ethTx.UnmarshalBinary(tx.Body); err != nil {
						fmt.Fprintf(os.Stderr, "warning: malformed 'evm.ethereum.v0' transaction %s in round %d: %v\n", tx.Hash(), round, err)
					}
					if ethTx.To() == nil {
						fmt.Fprintf(os.Stderr, "warning: transaction %s in round %d has nil 'to'\n", tx.Hash(), round)
					} else if txTo, _, err = helpers.ResolveEthOrOasisAddress(ethTx.To().String()); err != nil {
						fmt.Fprintf(os.Stderr, "warning: unable to parse 'to' address of transaction %s in round %d: %v\n", tx.Hash(), round, err)
					}

					txEthFrom, err := ethTypes.Sender(ethTypes.LatestSignerForChainID(ethTx.ChainId()), &ethTx)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: unable to decode 'from' address of transaction %s in round %d: %v\n", tx.Hash(), round, err)
					}
					txFrom, _, err = helpers.ResolveEthOrOasisAddress(txEthFrom.String())
				}
			} else {
				fmt.Fprintf(os.Stderr, "warning: unknown transaction %s in round %d\n", tx.Hash(), round)
			}

			for i, r := range t.Filters {
				if r.Match(txFrom, txTo) {
					t.DailyTxes[len(t.DailyTxes)-1][i+1]++
				}
			}

			// Sum up the tokens transferred by this transaction. Since a single
			// transaction may move tokens between multiple accounts (e.g. an EVM
			// call transferring the native token), we consider each emitted
			// accounts.Transfer event separately and match the filters against
			// the sender and the recipient of the event itself.
			for _, rawEv := range txr.Events {
				evs, err := accounts.DecodeEvent(rawEv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: malformed accounts event of transaction %s in round %d: %v\n", tx.Hash(), round, err)
					continue
				}
				for _, decodedEv := range evs {
					ev, ok := decodedEv.(*accounts.Event)
					if !ok || ev.Transfer == nil {
						continue
					}
					// Fees are not tokens transferred by the user.
					if ev.Transfer.To.Equal(accounts.FeeAccumulatorAddress) || ev.Transfer.From.Equal(accounts.FeeAccumulatorAddress) {
						continue
					}
					t.addAmount(0, ev.Transfer.Amount)
					for i, r := range t.Filters {
						if r.Match(&ev.Transfer.From, &ev.Transfer.To) {
							t.addAmount(i+1, ev.Transfer.Amount)
						}
					}
				}
			}
		}
		t.DailyTxes[len(t.DailyTxes)-1][0] += len(txes)
		round++
	}
	return nil
}

func (t *txStats) PrintStats() {
	c := csv.NewWriter(os.Stdout)
	c.WriteAll(t.CSVStats())
	c.Flush()
}

func (t *txStats) CSVStats() [][]string {
	csvData := make([][]string, len(t.Dates)+1)

	// Transaction count columns followed by the transferred amount columns of
	// each encountered denomination.
	csvData[0] = []string{"date", "all"}
	for _, r := range t.Filters {
		csvData[0] = append(csvData[0], r.Name)
	}
	for _, denom := range t.Denominations {
		symbol := t.paratime.GetDenominationInfo(string(denom)).Symbol
		csvData[0] = append(csvData[0], fmt.Sprintf("all amount (%s)", symbol))
		for _, r := range t.Filters {
			csvData[0] = append(csvData[0], fmt.Sprintf("%s amount (%s)", r.Name, symbol))
		}
	}

	for i, d := range t.DailyTxes {
		row := []string{t.Dates[i]}
		for _, dVal := range d {
			row = append(row, fmt.Sprintf("%d", dVal))
		}
		for _, denom := range t.Denominations {
			decimals := t.paratime.GetDenominationInfo(string(denom)).Decimals
			amounts, ok := t.DailyAmounts[i][denom]
			if !ok {
				amounts = make([]types.Quantity, len(t.Filters)+1)
			}
			for _, amount := range amounts {
				row = append(row, prettyprint.QuantityFrac(amount, decimals))
			}
		}
		csvData[i+1] = row
	}

	return csvData
}

func initLatestRound(ctx context.Context, rt connection.RuntimeClient) {
	var err error
	latestBlock, err = rt.GetBlock(ctx, client.RoundLatest)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("cannot get last retained block: %v", err))
	}
}

// computeFromToRounds parses fromStr and toStr and returns the corresponding round numbers.
func computeFromToRounds(rt connection.RuntimeClient) (from uint64, to uint64, err error) {
	switch {
	case fromStr == "":
		from = latestBlock.Header.Round
	case !strings.Contains(fromStr, "-"):
		if from, err = strconv.ParseUint(fromStr, 10, 64); err != nil {
			return
		}
	default:
		var fromDate time.Time
		fromDate, err = time.Parse(time.DateOnly, fromStr)
		if err != nil {
			err = fmt.Errorf("invalid from date format '%s': %v", fromStr, err)
			return
		}
		from = findRoundByDate(fromDate, rt)
	}

	switch {
	case toStr == "":
		to = latestBlock.Header.Round
	case !strings.Contains(toStr, "-"):
		if to, err = strconv.ParseUint(toStr, 10, 64); err != nil {
			return
		}
	default:
		var toDate time.Time
		toDate, err = time.Parse(time.DateOnly, toStr)
		if err != nil {
			err = fmt.Errorf("invalid to date format '%s': %v", toStr, err)
			return
		}
		to = findRoundByDate(toDate.AddDate(0, 0, 1), rt) - 1
	}

	return
}

// findStartRound is a helper that returns the earliest ParaTime block after the
// provided datetime point using the binary search.
func findRoundByDate(fromDate time.Time, rt connection.RuntimeClient) uint64 {
	const blockTime = 6 // TODO: Find the block time from runtime descriptor?
	ctx := context.Background()

	end := latestBlock.Header.Round
	start := end - (uint64((int64(latestBlock.Header.Timestamp)-fromDate.Unix())/blockTime) * 2)

	i := sort.Search(int(end-start), func(i int) bool {
		blk, err := rt.GetBlock(ctx, uint64(i)+start)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("cannot get block %d: %v", uint64(i)+start, err))
		}
		return int64(blk.Header.Timestamp) >= fromDate.Unix()
	})
	return min(uint64(i)+start, end)
}

var txStatsCmd = &cobra.Command{
	Use:     "transaction-statistics [filters]...",
	Example: "oasis pt tx-stats --network localhost_testnet --paratime sapphire --from 2024-01-31 --to 2024-01-31 -o 2024-01.csv  -- \"->0x973e69303259B0c2543a38665122b773D28405fB\"",
	Short:   "Generate daily transaction statistics",
	Long: "Produces daily transactions statistics for consensus or any ParaTime.\n" +
		"It connects to a client node directly and scrapes the blocks\n" +
		"corresponding to the given date range. Optionally, it filters out the\n" +
		"transactions satisfying from and/or to addresses.\n\n" +
		"Apart from the number of transactions, it also reports the amount of\n" +
		"tokens transferred by them, obtained from the emitted accounts transfer\n" +
		"events. The transaction fees are not included.",
	Aliases: []string{"tx-stats"},
	Run: func(cmd *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		var rt connection.RuntimeClient

		// Parse command line filters.
		var filters []*Filter
		for i := range args {
			f, err := newFilter(args[i], npa.Network)
			cobra.CheckErr(err)
			filters = append(filters, f)
		}

		// Connect to consensus/ParaTime client.
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		stats := newTxStats(filters, npa.ParaTime)
		var from, to uint64
		switch npa.ParaTime {
		case nil:
			// TODO: consensus
		default:
			rt = conn.Runtime(npa.ParaTime)
			initLatestRound(ctx, rt)

			from, to, err = computeFromToRounds(rt)
			cobra.CheckErr(err)
			fmt.Fprintf(os.Stderr, "Generating statistics from round %d to %d\n", from, to)

			err = stats.AnalyzeRuntimeBlocks(from, to, rt)
			cobra.CheckErr(err)
		}

		if fileCSV == "" {
			stats.PrintStats()
			return
		}

		// Also save entity stats in a csv.
		fout, err := os.Create(fileCSV)
		cobra.CheckErr(err)
		defer fout.Close()

		w := csv.NewWriter(fout)
		err = w.WriteAll(stats.CSVStats())
		cobra.CheckErr(err)
	},
}

func init() {
	txStatsCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	txStatsCmd.Flags().StringVarP(&fileCSV, "output-file", "o", "", "output statistics into specified CSV file")
	txStatsCmd.Flags().StringVar(&fromStr, "from", "", "from date in YYYY-MM-DD format or block/round number")
	txStatsCmd.Flags().StringVar(&toStr, "to", "", "to (inclusive) date in YYYY-MM-DD format or block/round number")
}
