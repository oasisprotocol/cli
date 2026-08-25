package provider

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

// testAddresses maps the account names understood by testResolver to their addresses.
var testAddresses = map[string]string{
	"alice": "oasis1qrec770vrek0a9a5lcrv0zvt22504k68svq7kzve",
	"bob":   "oasis1qrydpazemvuwtnp3efm7vmfvg3tde044qg6cxwzx",
}

// testResolver resolves the account names in testAddresses and any valid Oasis address.
func testResolver(nameOrAddress string) (types.Address, error) {
	if addr, ok := testAddresses[nameOrAddress]; ok {
		nameOrAddress = addr
	}

	var addr types.Address
	if err := addr.UnmarshalText([]byte(nameOrAddress)); err != nil {
		return types.Address{}, fmt.Errorf("unsupported address format")
	}
	return addr, nil
}

const (
	testHashA = "1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a"
	testHashB = "2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b"
)

func newTestOffer() *Offer {
	return &Offer{
		ID: "small",
		Resources: Resources{
			TEE:      "tdx",
			Memory:   1024,
			CPUCount: 1,
			Storage:  2048,
		},
		Payment: Payment{
			Native: &NativePayment{
				Terms: map[string]string{TermKeyHour: "10"},
			},
		},
		Capacity: 1,
	}
}

func TestOfferGetMetadataAccessPolicy(t *testing.T) {
	require := require.New(t)

	offer := newTestOffer()
	offer.AllowedCreators = []string{"bob", testAddresses["alice"]}
	offer.AllowedArtifacts = map[string][]string{
		"firmware": {testHashB, testHashA},
		"kernel":   {testHashA},
	}
	offer.Private = true

	meta, err := offer.GetMetadata(testResolver)
	require.NoError(err)

	require.Equal("small", meta[SchedulerMetadataOfferKey])
	// Entries must be sorted so that reordering them in the manifest is not seen as a change.
	require.Equal(
		testAddresses["alice"]+","+testAddresses["bob"],
		meta[SchedulerMetadataOfferAllowedCreatorsKey],
	)
	require.Equal(testHashA+","+testHashB, meta[SchedulerMetadataOfferAllowedArtifactsPrefix+"firmware"])
	require.Equal(testHashA, meta[SchedulerMetadataOfferAllowedArtifactsPrefix+"kernel"])
	require.Equal(SchedulerMetadataValueTrue, meta[SchedulerMetadataOfferPrivateKey])
}

func TestOfferGetMetadataAccessPolicyOmitted(t *testing.T) {
	require := require.New(t)

	meta, err := newTestOffer().GetMetadata(testResolver)
	require.NoError(err)

	// Absent policy must not emit any keys, otherwise everyone's offers would be updated on-chain.
	require.NotContains(meta, SchedulerMetadataOfferAllowedCreatorsKey)
	require.NotContains(meta, SchedulerMetadataOfferAllowedArtifactsPrefix+"firmware")
	require.NotContains(meta, SchedulerMetadataOfferPrivateKey)
}

func TestOfferGetMetadataDeduplicatesCreators(t *testing.T) {
	require := require.New(t)

	offer := newTestOffer()
	// The same account, once by name and once by address.
	offer.AllowedCreators = []string{"alice", testAddresses["alice"]}

	meta, err := offer.GetMetadata(testResolver)
	require.NoError(err)
	require.Equal(testAddresses["alice"], meta[SchedulerMetadataOfferAllowedCreatorsKey])
}

func TestOfferGetMetadataExplicitOverride(t *testing.T) {
	require := require.New(t)

	offer := newTestOffer()
	offer.Private = true
	offer.Metadata = map[string]string{
		SchedulerMetadataOfferPrivateKey:                          "0",
		SchedulerMetadataOfferAllowedArtifactsPrefix + "firmware": testHashA,
	}

	meta, err := offer.GetMetadata(testResolver)
	require.NoError(err)
	require.Equal("0", meta[SchedulerMetadataOfferPrivateKey])
	require.Equal(testHashA, meta[SchedulerMetadataOfferAllowedArtifactsPrefix+"firmware"])
}

func TestOfferGetMetadataUnresolvableCreator(t *testing.T) {
	offer := newTestOffer()
	offer.AllowedCreators = []string{"nonexistent"}

	_, err := offer.GetMetadata(testResolver)
	require.ErrorContains(t, err, "invalid allowed creator 'nonexistent'")
}

func TestOfferValidateAccessPolicy(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Offer)
		errMsg string
	}{
		{"Valid", func(o *Offer) {
			o.AllowedCreators = []string{"alice"}
			o.AllowedArtifacts = map[string][]string{"stage2": {testHashA}}
			o.Private = true
		}, ""},
		{"EmptyCreator", func(o *Offer) {
			o.AllowedCreators = []string{"  "}
		}, "empty account"},
		{"CommaInCreator", func(o *Offer) {
			o.AllowedCreators = []string{"alice,bob"}
		}, "must not contain a comma"},
		{"UnknownArtifactKind", func(o *Offer) {
			o.AllowedArtifacts = map[string][]string{"bios": {testHashA}}
		}, "invalid allowed artifact kind 'bios'"},
		{"NonHexArtifactHash", func(o *Offer) {
			o.AllowedArtifacts = map[string][]string{"initrd": {"not-a-hash"}}
		}, "malformed hash"},
		{"ShortArtifactHash", func(o *Offer) {
			o.AllowedArtifacts = map[string][]string{"initrd": {"1a2b"}}
		}, "expected 32 bytes, got 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			offer := newTestOffer()
			tc.mutate(offer)

			err := offer.Validate()
			if tc.errMsg == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.errMsg)
		})
	}
}

func TestIsOfferPrivate(t *testing.T) {
	require := require.New(t)

	require.False(IsOfferPrivate(&roflmarket.Offer{}))
	require.False(IsOfferPrivate(&roflmarket.Offer{
		Metadata: map[string]string{SchedulerMetadataOfferPrivateKey: "0"},
	}))
	// Only the exact "1" value enables the flag, matching the scheduler.
	require.False(IsOfferPrivate(&roflmarket.Offer{
		Metadata: map[string]string{SchedulerMetadataOfferPrivateKey: "true"},
	}))
	require.True(IsOfferPrivate(&roflmarket.Offer{
		Metadata: map[string]string{SchedulerMetadataOfferPrivateKey: SchedulerMetadataValueTrue},
	}))
}

func TestOfferAllowedCreators(t *testing.T) {
	require := require.New(t)

	alice, bob := testAddresses["alice"], testAddresses["bob"]

	require.Empty(OfferAllowedCreators(&roflmarket.Offer{}))
	require.Empty(OfferAllowedCreators(&roflmarket.Offer{
		Metadata: map[string]string{SchedulerMetadataOfferAllowedCreatorsKey: ""},
	}))
	require.Equal([]string{alice, bob}, OfferAllowedCreators(&roflmarket.Offer{
		Metadata: map[string]string{
			SchedulerMetadataOfferAllowedCreatorsKey: alice + "," + bob,
		},
	}))
	// Stray whitespace and empty items are ignored.
	require.Equal([]string{alice}, OfferAllowedCreators(&roflmarket.Offer{
		Metadata: map[string]string{
			SchedulerMetadataOfferAllowedCreatorsKey: " " + alice + " ,, ",
		},
	}))
}
