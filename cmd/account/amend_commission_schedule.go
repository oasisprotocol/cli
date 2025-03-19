package account

import (
	"context"
	"fmt"
	"math/big"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	commissionScheduleRates  []string
	commissionScheduleBounds []string

	amendCommissionScheduleCmd = &cobra.Command{
		Use:   "amend-commission-schedule",
		Short: "Amend the validator's commission schedule",
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			npa.MustHaveAccount()
			acc := common.LoadAccount(cfg, npa.AccountName)

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var (
				conn connection.Connection

				rules    *staking.CommissionScheduleRules
				schedule *staking.CommissionSchedule
				now      beacon.EpochTime
			)
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)

				// And also query the various dynamic values required
				// to validate the amendment.

				var height int64
				height, err = common.GetActualHeight(
					ctx,
					conn.Consensus(),
				)
				cobra.CheckErr(err)

				now, err = conn.Consensus().Beacon().GetEpoch(ctx, height)
				cobra.CheckErr(err)

				addr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, npa.Account.Address)
				cobra.CheckErr(err)

				stakingConn := conn.Consensus().Staking()

				params, err := stakingConn.ConsensusParameters(ctx, height)
				cobra.CheckErr(err)

				consensusAccount, err := stakingConn.Account(
					ctx,
					&staking.OwnerQuery{
						Owner:  addr.ConsensusAddress(),
						Height: height,
					},
				)
				cobra.CheckErr(err)

				rules = &params.CommissionScheduleRules
				schedule = &consensusAccount.Escrow.CommissionSchedule
			}

			var amendment staking.AmendCommissionSchedule
			if rawRates := commissionScheduleRates; len(rawRates) > 0 {
				amendment.Amendment.Rates = make([]staking.CommissionRateStep, len(rawRates))
				for i, rawRate := range rawRates {
					if err := scanRateStep(&amendment.Amendment.Rates[i], rawRate); err != nil {
						cobra.CheckErr(fmt.Errorf("failed to parse commission schedule rate step %d: %w", i, err))
					}
				}
			}
			if rawBounds := commissionScheduleBounds; len(rawBounds) > 0 {
				amendment.Amendment.Bounds = make([]staking.CommissionRateBoundStep, len(rawBounds))
				for i, rawBound := range rawBounds {
					if err := scanBoundStep(&amendment.Amendment.Bounds[i], rawBound); err != nil {
						cobra.CheckErr(fmt.Errorf("failed to parse commission schedule bound step %d: %w", i, err))
					}
				}
			}

			if rules != nil && schedule != nil {
				// If we are in online mode, try to validate the amendment.
				err := schedule.AmendAndPruneAndValidate(
					&amendment.Amendment,
					rules,
					now,
				)
				cobra.CheckErr(err)
			}

			// Prepare transaction.
			tx := staking.NewAmendCommissionScheduleTx(0, nil, &amendment)

			sigTx, err := common.SignConsensusTransaction(ctx, npa, acc, conn, tx)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, nil, nil)
		},
	}
)

func scanRateStep(
	dst *staking.CommissionRateStep,
	raw string,
) error {
	var rateBI big.Int
	n, err := fmt.Sscanf(raw, "%d/%d", &dst.Start, &rateBI)
	if err != nil {
		return err
	}
	if n != 2 {
		return fmt.Errorf("scanned %d values (need 2)", n)
	}
	if err = dst.Rate.FromBigInt(&rateBI); err != nil {
		return fmt.Errorf("rate: %w", err)
	}
	return nil
}

func scanBoundStep(
	dst *staking.CommissionRateBoundStep,
	raw string,
) error {
	var (
		rateMinBI big.Int
		rateMaxBI big.Int
	)
	n, err := fmt.Sscanf(raw, "%d/%d/%d", &dst.Start, &rateMinBI, &rateMaxBI)
	if err != nil {
		return err
	}
	if n != 3 {
		return fmt.Errorf("scanned %d values (need 3)", n)
	}
	if err = dst.RateMin.FromBigInt(&rateMinBI); err != nil {
		return fmt.Errorf("rate min: %w", err)
	}

	if err = dst.RateMax.FromBigInt(&rateMaxBI); err != nil {
		return fmt.Errorf("rate max: %w", err)
	}
	return nil
}

func init() {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.StringSliceVar(&commissionScheduleRates, "rates", nil, fmt.Sprintf(
		"commission rate step. Multiple of this flag is allowed. "+
			"Each step is in the format start_epoch/rate_numerator. "+
			"The rate is rate_numerator divided by %v", staking.CommissionRateDenominator,
	))
	f.StringSliceVar(&commissionScheduleBounds, "bounds", nil, fmt.Sprintf(
		"commission rate bound step. Multiple of this flag is allowed. "+
			"Each step is in the format start_epoch/rate_min_numerator/rate_max_numerator. "+
			"The minimum rate is rate_min_numerator divided by %v, and the maximum rate is "+
			"rate_max_numerator divided by %v", staking.CommissionRateDenominator, staking.CommissionRateDenominator,
	))
	amendCommissionScheduleCmd.Flags().AddFlagSet(common.SelectorNAFlags)
	amendCommissionScheduleCmd.Flags().AddFlagSet(common.TxFlags)
	amendCommissionScheduleCmd.Flags().AddFlagSet(f)
}
