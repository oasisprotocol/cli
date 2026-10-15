package rofl

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
)

func TestReplaceUnavailableMachine(t *testing.T) {
	queryFailure := errors.New("query failed")
	rentFailure := errors.New("rental failed")
	missing := errors.New("roflmarket: instance not found")
	// time.Time stores seconds relative to year 1 instead of the Unix epoch.
	const unixToInternalSeconds = 62135596800
	for _, tc := range []struct {
		name       string
		paidUntil  uint64
		now        time.Time
		queryErr   error
		replace    bool
		rentErr    error
		wantRent   bool
		wantErr    error
		wantErrMsg string
	}{
		{name: "unexpired", paidUntil: 100, now: time.Unix(99, 999999999), replace: true},
		{name: "unexpired without replacement", paidUntil: 100, now: time.Unix(99, 0)},
		{name: "before internal time overflow", paidUntil: math.MaxInt64 - unixToInternalSeconds - 1, now: time.Unix(101, 0), replace: true},
		{name: "internal time overflow boundary", paidUntil: math.MaxInt64 - unixToInternalSeconds, now: time.Unix(101, 0), replace: true},
		{name: "after internal time overflow", paidUntil: math.MaxInt64 - unixToInternalSeconds + 1, now: time.Unix(101, 0), replace: true},
		{name: "maximum signed timestamp", paidUntil: math.MaxInt64, now: time.Unix(101, 0), replace: true},
		{name: "before epoch", paidUntil: 100, now: time.Unix(-1, 999999999), replace: true},
		{name: "zero timestamp before epoch", paidUntil: 0, now: time.Unix(-1, 999999999), replace: true},
		{name: "zero timestamp at epoch", paidUntil: 0, now: time.Unix(0, 0), replace: true, wantRent: true},
		{name: "zero timestamp after epoch", paidUntil: 0, now: time.Unix(0, 1), wantErrMsg: "machine instance expired; use --replace-machine"},
		{name: "beyond signed timestamp range", paidUntil: math.MaxInt64 + 1, now: time.Unix(101, 0), replace: true},
		{name: "maximum timestamp", paidUntil: math.MaxUint64, now: time.Unix(101, 0)},
		{name: "expiration boundary", paidUntil: 100, now: time.Unix(100, 0), replace: true, wantRent: true},
		{name: "expired accepted", paidUntil: 100, now: time.Unix(101, 0), replace: true, wantRent: true},
		{name: "expired without replacement", paidUntil: 100, now: time.Unix(101, 0), wantErrMsg: "machine instance expired; use --replace-machine"},
		{name: "missing", paidUntil: 100, queryErr: missing, replace: true, wantRent: true},
		{name: "missing without replacement", paidUntil: 100, queryErr: missing, wantErrMsg: "machine instance not found; use --replace-machine"},
		{name: "query failure", paidUntil: 100, queryErr: queryFailure, replace: true, wantErr: queryFailure},
		{name: "expired rental failure", paidUntil: 100, now: time.Unix(100, 0), replace: true, rentErr: rentFailure, wantRent: true, wantErr: rentFailure},
		{name: "missing rental failure", paidUntil: 100, queryErr: missing, replace: true, rentErr: rentFailure, wantRent: true, wantErr: rentFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := &roflmarket.Instance{PaidUntil: tc.paidUntil, Status: roflmarket.InstanceStatusAccepted}
			instance := original
			if tc.queryErr != nil {
				instance = nil
			}
			replacement := &roflmarket.Instance{PaidUntil: 200}
			rentals := 0
			got, replaced, err := replaceUnavailableMachine(instance, tc.queryErr, tc.now, tc.replace, func() (*roflmarket.Instance, error) {
				rentals++
				if tc.rentErr != nil {
					return nil, tc.rentErr
				}
				return replacement, nil
			})
			require.Equal(t, tc.wantRent, replaced)
			if tc.wantRent {
				require.Equal(t, 1, rentals)
			} else {
				require.Zero(t, rentals)
			}
			switch {
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
				require.Nil(t, got)
			case tc.wantErrMsg != "":
				require.ErrorContains(t, err, tc.wantErrMsg)
				require.Nil(t, got)
			default:
				require.NoError(t, err)
				if tc.wantRent {
					require.Same(t, replacement, got)
				} else {
					require.Same(t, original, got)
				}
			}
		})
	}
}
