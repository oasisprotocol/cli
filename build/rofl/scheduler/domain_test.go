package scheduler

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

func testAppID(t *testing.T, raw string) rofl.AppID {
	t.Helper()

	var appID rofl.AppID
	require.NoError(t, appID.UnmarshalText([]byte(raw)), "app id must parse")
	return appID
}

func testInstance(t *testing.T, provider string, id string) *roflmarket.Instance {
	t.Helper()

	var providerAddr types.Address
	require.NoError(t, providerAddr.UnmarshalText([]byte(provider)), "provider address must parse")

	rawID, err := hex.DecodeString(id)
	require.NoError(t, err, "instance id must parse")
	require.Len(t, rawID, 8, "instance id must be 8 bytes")

	instance := &roflmarket.Instance{Provider: providerAddr}
	copy(instance.ID[:], rawID)
	return instance
}

// TestDomainVerificationToken pins the derivation against a token that a live
// rofl-scheduler (v0.9.0) accepted, so any change here that breaks compatibility with
// the scheduler's Rust implementation shows up as a test failure.
func TestDomainVerificationToken(t *testing.T) {
	instance := testInstance(t, "oasis1qp2ens0hsp7gh23wajxa4hpetkdek3swyyulyrmz", "0000000000000634")
	deployedApp := testAppID(t, "rofl1qrmnjkx47f4tcfvfclnrtj2rad82akeum5jcpe8y")

	require.Equal(t,
		"ofdkQl2NWpILtYBJBQC+rfQj68oyKgfkuu0PyJlKGYk=",
		DomainVerificationToken(instance, deployedApp, "api.testnet.privana.finance"),
	)
}

// TestDomainVerificationTokenBoundToDeployedApp ensures the token is bound to the app
// deployed on the machine. Deriving it from the provider's scheduler app instead
// yields a token the scheduler will never verify.
func TestDomainVerificationTokenBoundToDeployedApp(t *testing.T) {
	instance := testInstance(t, "oasis1qp2ens0hsp7gh23wajxa4hpetkdek3swyyulyrmz", "0000000000000634")
	deployedApp := testAppID(t, "rofl1qrmnjkx47f4tcfvfclnrtj2rad82akeum5jcpe8y")
	schedulerApp := testAppID(t, "rofl1qrqw99h0f7az3hwt2cl7yeew3wtz0fxunu7luyfg")

	const domain = "api.testnet.privana.finance"

	require.NotEqual(t,
		DomainVerificationToken(instance, deployedApp, domain),
		DomainVerificationToken(instance, schedulerApp, domain),
		"token must change with the app it is derived from",
	)
}
