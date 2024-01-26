package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
)

func TestResolveAddress(t *testing.T) {
	require := require.New(t)

	net := config.Network{
		ParaTimes: config.ParaTimes{
			All: map[string]*config.ParaTime{
				"pt1": {
					ID: "0000000000000000000000000000000000000000000000000000000000000000",
				},
			},
		},
	}

	for _, tc := range []struct {
		address         string
		expectedAddr    string
		expectedEthAddr string
	}{
		{"", "", ""},
		{"oasis1", "", ""},
		{"oasis1blah", "", ""},
		{"oasis1qqzh32kr72v7x55cjnjp2me0pdn579u6as38kacz", "oasis1qqzh32kr72v7x55cjnjp2me0pdn579u6as38kacz", ""},
		{"0x", "", ""},
		{"0xblah", "", ""},
		{"0x60a6321eA71d37102Dbf923AAe2E08d005C4e403", "oasis1qpaqumrpewltmh9mr73hteycfzveus2rvvn8w5sp", "0x60a6321eA71d37102Dbf923AAe2E08d005C4e403"},
		{"paratime:", "", ""},
		{"paratime:invalid", "", ""},
		{"paratime:pt1", "oasis1qqdn25n5a2jtet2s5amc7gmchsqqgs4j0qcg5k0t", ""},
		{"pool:", "", ""},
		{"pool:invalid", "", ""},
		{"pool:paratime:common", "oasis1qz78phkdan64g040cvqvqpwkplfqf6tj6uwcsh30", ""},
		{"pool:paratime:fee-accumulator", "oasis1qp3r8hgsnphajmfzfuaa8fhjag7e0yt35cjxq0u4", ""},
		{"pool:paratime:rewards", "oasis1qp7x0q9qahahhjas0xde8w0v04ctp4pqzu5mhjav", ""},
		{"pool:paratime:pending-delegation", "oasis1qzcdegtf7aunxr5n5pw7n5xs3u7cmzlz9gwmq49r", ""},
		{"pool:paratime:pending-withdrawal", "oasis1qr677rv0dcnh7ys4yanlynysvnjtk9gnsyhvm6ln", ""},
		{"pool:consensus:burn", "oasis1qzq8u7xs328puu2jy524w3fygzs63rv3u5967970", ""},
		{"pool:consensus:common", "oasis1qrmufhkkyyf79s5za2r8yga9gnk4t446dcy3a5zm", ""},
		{"pool:consensus:fee-accumulator", "oasis1qqnv3peudzvekhulf8v3ht29z4cthkhy7gkxmph5", ""},
		{"pool:consensus:governance-deposits", "oasis1qp65laz8zsa9a305wxeslpnkh9x4dv2h2qhjz0ec", ""},
		{"test:alice", "oasis1qrec770vrek0a9a5lcrv0zvt22504k68svq7kzve", ""},
		{"test:dave", "oasis1qrk58a6j2qn065m6p06jgjyt032f7qucy5wqeqpt", "0xDce075E1C39b1ae0b75D554558b6451A226ffe00"},
		{"test:frank", "oasis1qqnf0s9p8z79zfutszt0hwlh7w7jjrfqnq997mlw", ""},
		{"test:invalid", "", ""},
		{"invalid:", "", ""},
	} {
		addr, ethAddr, err := ResolveAddress(&net, tc.address)
		if len(tc.expectedAddr) > 0 {
			require.NoError(err, tc.address)
			require.EqualValues(tc.expectedAddr, addr.String(), tc.address)
			if len(tc.expectedEthAddr) > 0 {
				require.EqualValues(tc.expectedEthAddr, ethAddr.String())
			}
		} else {
			require.Error(err, tc.address)
		}
	}
}

func TestParseTestAccountAddress(t *testing.T) {
	require := require.New(t)

	for _, tc := range []struct {
		address  string
		expected string
	}{
		{"test:abc", "abc"},
		{"testabc", ""},
		{"testing:abc", ""},
		{"oasis1qqzh32kr72v7x55cjnjp2me0pdn579u6as38kacz", ""},
		{"", ""},
	} {
		testName := ParseTestAccountAddress(tc.address)
		require.EqualValues(tc.expected, testName, tc.address)
	}
}
