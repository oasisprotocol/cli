package show

import (
	"context"
	"fmt"
	"io"
	"sort"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/cmd/common"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

const amountFieldName = "Amount:"

// lenLongestString returns the length of the longest string passed to it.
func lenLongestString(strs ...string) int {
	maxLen := 0
	for _, s := range strs {
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}
	return maxLen
}

// delegationDescription is a description of a (debonding) delegation.
type delegationDescription struct {
	address staking.Address
	self    bool
	amount  types.Quantity
	shares  types.Quantity
	endTime beacon.EpochTime
}

// byEndTimeAmountAddress sorts the delegationDescription list by:
// 1. increasing end time (only applicable to debonding delegations),
// 2. decreasing amount,
// 3. increasing address.
//
// Later criteria is only applicable when multiple delegations are equal
// according to preceding criteria.
type byEndTimeAmountAddress []delegationDescription

func (a byEndTimeAmountAddress) Len() int {
	return len(a)
}

func (a byEndTimeAmountAddress) Less(i, j int) bool {
	if a[i].endTime == a[j].endTime {
		if a[i].amount.Cmp(&a[j].amount) == 0 {
			return a[i].address.String() < a[j].address.String()
		}
		return a[i].amount.Cmp(&a[j].amount) > 0
	}
	return a[i].endTime < a[j].endTime
}

func (a byEndTimeAmountAddress) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// delegationAmount returns the number of base units the given amount of shares
// represent in the given share pool.
func delegationAmount(shares types.Quantity, sharePool staking.SharePool) types.Quantity {
	amount, _ := sharePool.StakeForShares(&shares)
	return *amount
}

// prettyPrintDelegationDescriptions pretty-prints the given list of delegation
// descriptions.
func prettyPrintDelegationDescriptions(
	network *config.Network,
	delDescriptions []delegationDescription,
	addressFieldName string,
	prefix string,
	w io.Writer,
) {
	const endTimeFieldName = "End Time:"

	fmt.Fprintf(w, "%sDelegations:\n", prefix)

	sort.Sort(byEndTimeAmountAddress(delDescriptions))

	// Get the length of name of the longest field to display for each
	// element so we can align all values.
	// NOTE: We assume the delegation descriptions are either all for
	// (active) delegations or all for debonding delegations.
	lenLongest := 0
	if delDescriptions[0].endTime == beacon.EpochInvalid {
		// Active delegations.
		lenLongest = lenLongestString(addressFieldName, amountFieldName)
	} else {
		// Debonding delegations.
		lenLongest = lenLongestString(addressFieldName, amountFieldName, endTimeFieldName)
	}

	for _, desc := range delDescriptions {
		fmt.Fprintf(w, "%s  - %-*s %s", prefix, lenLongest, addressFieldName, desc.address)
		if desc.self {
			fmt.Fprintf(w, " (self)")
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s    %-*s ", prefix, lenLongest, amountFieldName)
		fmt.Fprintf(w, "%s", helpers.FormatConsensusDenomination(network, desc.amount))
		fmt.Fprintf(w, " (%s shares)\n", desc.shares)
		if desc.endTime != beacon.EpochInvalid {
			fmt.Fprintf(w, "%s    %-*s epoch %d\n", prefix, lenLongest, endTimeFieldName, desc.endTime)
		}
	}
}

// prettyPrintAccountBalanceAndDelegationsFrom pretty-prints the given account's general balance and
// (outgoing) delegations from this account.
func prettyPrintAccountBalanceAndDelegationsFrom(
	network *config.Network,
	addr *types.Address,
	generalAccount staking.GeneralAccount,
	actDelegationInfos map[staking.Address]*staking.DelegationInfo,
	debDelegationInfos map[staking.Address][]*staking.DebondingDelegationInfo,
	prefix string,
	w io.Writer,
) {
	var totalActDelegationsAmount, totalDebDelegationsAmount types.Quantity

	consensusAddr := addr.ConsensusAddress()
	availableAmount := generalAccount.Balance
	totalAmount := availableAmount.Clone()

	actDelegationDescs := make([]delegationDescription, 0, len(actDelegationInfos))

	for delAddr, delInfo := range actDelegationInfos {
		delDesc := delegationDescription{
			delAddr,
			delAddr.Equal(consensusAddr),
			delegationAmount(delInfo.Shares, delInfo.Pool),
			delInfo.Shares,
			beacon.EpochInvalid,
		}
		actDelegationDescs = append(actDelegationDescs, delDesc)
		_ = totalActDelegationsAmount.Add(&delDesc.amount)
	}
	_ = totalAmount.Add(&totalActDelegationsAmount)

	debDelegationDescs := make([]delegationDescription, 0, len(debDelegationInfos))

	for delAddr, delInfoList := range debDelegationInfos {
		for _, delInfo := range delInfoList {
			delDesc := delegationDescription{
				delAddr,
				delAddr.Equal(consensusAddr),
				delegationAmount(delInfo.Shares, delInfo.Pool),
				delInfo.Shares,
				delInfo.DebondEndTime,
			}
			debDelegationDescs = append(debDelegationDescs, delDesc)
			_ = totalDebDelegationsAmount.Add(&delDesc.amount)
		}
	}
	_ = totalAmount.Add(&totalDebDelegationsAmount)

	fmt.Fprintf(w, "%sTotal: ", prefix)
	fmt.Fprintf(w, "%s\n", helpers.FormatConsensusDenomination(network, *totalAmount))
	fmt.Fprintf(w, "%sAvailable: ", prefix)
	fmt.Fprintf(w, "%s\n", helpers.FormatConsensusDenomination(network, availableAmount))
	fmt.Fprintln(w)

	innerPrefix := prefix + "  "
	const addressFieldName = "To:"

	if len(actDelegationDescs) > 0 {
		fmt.Fprintf(w, "%sActive Delegations from this Account:\n", prefix)
		fmt.Fprintf(w, "%sTotal: ", innerPrefix)
		fmt.Fprintf(w, "%s\n", helpers.FormatConsensusDenomination(network, totalActDelegationsAmount))
		fmt.Fprintln(w)

		sort.Sort(byEndTimeAmountAddress(actDelegationDescs))
		prettyPrintDelegationDescriptions(network, actDelegationDescs, addressFieldName, innerPrefix, w)
		fmt.Fprintln(w)
	}

	if len(debDelegationDescs) > 0 {
		fmt.Fprintf(w, "%sDebonding Delegations from this Account:\n", prefix)
		fmt.Fprintf(w, "%sTotal: ", innerPrefix)
		fmt.Fprintf(w, "%s\n", helpers.FormatConsensusDenomination(network, totalDebDelegationsAmount))
		fmt.Fprintln(w)

		sort.Sort(byEndTimeAmountAddress(debDelegationDescs))
		prettyPrintDelegationDescriptions(network, debDelegationDescs, addressFieldName, innerPrefix, w)
		fmt.Fprintln(w)
	}
}

// prettyPrintDelegationsTo pretty-prints the given incoming (debonding) delegations to the given
// escrow account.
func prettyPrintDelegationsTo(
	network *config.Network,
	addr *types.Address,
	sharePool staking.SharePool,
	delegations interface{},
	prefix string,
	w io.Writer,
) {
	consensusAddr := addr.ConsensusAddress()

	var delDescs []delegationDescription

	switch dels := delegations.(type) {
	case map[staking.Address]*staking.Delegation:
		for delAddr, del := range dels {
			delDesc := delegationDescription{
				delAddr,
				delAddr.Equal(consensusAddr),
				delegationAmount(del.Shares, sharePool),
				del.Shares,
				beacon.EpochInvalid,
			}
			delDescs = append(delDescs, delDesc)
		}
	case map[staking.Address][]*staking.DebondingDelegation:
		for delAddr, delList := range dels {
			for _, del := range delList {
				delDesc := delegationDescription{
					delAddr,
					delAddr.Equal(consensusAddr),
					delegationAmount(del.Shares, sharePool),
					del.Shares,
					del.DebondEndTime,
				}
				delDescs = append(delDescs, delDesc)
			}
		}
	default:
		fmt.Fprintf(w, "%sERROR: Unsupported delegations type: %T)\n", prefix, dels)
		return
	}

	fmt.Fprintf(w, "%sTotal: ", prefix)
	fmt.Fprintf(w, "%s", helpers.FormatConsensusDenomination(network, sharePool.Balance))
	fmt.Fprintf(w, " (%s shares)\n", sharePool.TotalShares)
	fmt.Fprintln(w)

	const addressFieldName = "From:"

	sort.Sort(byEndTimeAmountAddress(delDescs))
	prettyPrintDelegationDescriptions(network, delDescs, addressFieldName, prefix, w)
}

func prettyPrintParaTimeDelegations(
	ctx context.Context,
	c connection.Connection,
	height int64,
	npa *common.NPASelection,
	addr *types.Address,
	rtDelegations []*consensusaccounts.ExtendedDelegationInfo,
	rtUndelegations []*consensusaccounts.UndelegationInfo,
	prefix string,
	w io.Writer,
) {
	var totalDeg quantity.Quantity
	delegations := make([]delegationDescription, 0, len(rtDelegations))
	for _, di := range rtDelegations {
		// For each destination we need to fetch the pool.
		destAccount, err := c.Consensus().Staking().Account(ctx, &staking.OwnerQuery{
			Owner:  di.To.ConsensusAddress(),
			Height: height,
		})
		cobra.CheckErr(err)

		// Then we can compute the current amount.
		amount, _ := destAccount.Escrow.Active.StakeForShares(&di.Shares)
		_ = totalDeg.Add(amount)

		delegations = append(delegations, delegationDescription{
			address: di.To.ConsensusAddress(),
			self:    di.To.Equal(*addr),
			amount:  *amount,
			shares:  di.Shares,
			endTime: beacon.EpochInvalid,
		})
	}

	var totalUndeg quantity.Quantity
	undelegations := make([]delegationDescription, 0, len(rtUndelegations))
	for _, udi := range rtUndelegations {
		// For each destination we need to fetch the pool.
		destAccount, err := c.Consensus().Staking().Account(ctx, &staking.OwnerQuery{
			Owner:  udi.From.ConsensusAddress(),
			Height: height,
		})
		cobra.CheckErr(err)

		// Then we can compute the current amount.
		amount, _ := destAccount.Escrow.Debonding.StakeForShares(&udi.Shares)
		_ = totalUndeg.Add(amount)

		undelegations = append(undelegations, delegationDescription{
			address: udi.From.ConsensusAddress(),
			self:    udi.From.Equal(*addr),
			amount:  *amount,
			shares:  udi.Shares,
			endTime: udi.Epoch,
		})
	}

	innerPrefix := prefix + "  "

	if len(delegations) > 0 {
		fmt.Fprintf(w, "%sActive Delegations from this Account:\n", prefix)
		fmt.Fprintf(w, "%sTotal: %s\n", innerPrefix, helpers.FormatConsensusDenomination(npa.Network, totalDeg))
		fmt.Fprintln(w)

		sort.Sort(byEndTimeAmountAddress(delegations))
		prettyPrintDelegationDescriptions(npa.Network, delegations, "To:", innerPrefix, w)
		fmt.Fprintln(w)
	}

	if len(undelegations) > 0 {
		fmt.Fprintf(w, "%sDebonding Delegations from this Account:\n", prefix)
		fmt.Fprintf(w, "%sTotal: %s\n", innerPrefix, helpers.FormatConsensusDenomination(npa.Network, totalUndeg))
		fmt.Fprintln(w)

		sort.Sort(byEndTimeAmountAddress(undelegations))
		prettyPrintDelegationDescriptions(npa.Network, undelegations, "To:", innerPrefix, w)
		fmt.Fprintln(w)
	}
}
