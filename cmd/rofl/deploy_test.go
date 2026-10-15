package rofl

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
)

func TestReplaceUnavailableMachine(t *testing.T) {
	queryFailure := errors.New("query failed")
	rentFailure := errors.New("rental failed")
	missing := errors.New("roflmarket: instance not found")
	for _, tc := range []struct {
		name       string
		now        time.Time
		queryErr   error
		replace    bool
		rentErr    error
		wantRent   bool
		wantErr    error
		wantErrMsg string
	}{
		{name: "unexpired", now: time.Unix(99, 999999999), replace: true},
		{name: "unexpired without replacement", now: time.Unix(99, 0)},
		{name: "expiration boundary", now: time.Unix(100, 0), replace: true, wantRent: true},
		{name: "expired accepted", now: time.Unix(101, 0), replace: true, wantRent: true},
		{name: "expired without replacement", now: time.Unix(101, 0), wantErrMsg: "machine instance expired; use --replace-machine"},
		{name: "missing", queryErr: missing, replace: true, wantRent: true},
		{name: "missing without replacement", queryErr: missing, wantErrMsg: "machine instance not found; use --replace-machine"},
		{name: "query failure", queryErr: queryFailure, replace: true, wantErr: queryFailure},
		{name: "expired rental failure", now: time.Unix(100, 0), replace: true, rentErr: rentFailure, wantRent: true, wantErr: rentFailure},
		{name: "missing rental failure", queryErr: missing, replace: true, rentErr: rentFailure, wantRent: true, wantErr: rentFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := &roflmarket.Instance{PaidUntil: 100, Status: roflmarket.InstanceStatusAccepted}
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
