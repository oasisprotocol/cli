package file //nolint: dupl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSr25519FromMnemonic(t *testing.T) {
	mnemonics := []struct {
		mnemonic string
		num      uint32
		pubkey   string
		valid    bool
	}{
		{mnemonic: "equip will roof matter pink blind book anxiety banner elbow sun young", num: 0, pubkey: "vP1R5MMzR/r9ricymYpPbBA9JvmlixdzlEYWEpx5ExY=", valid: true},
		{mnemonic: "equip will roof matter pink blind book anxiety banner elbow sun young", num: 1, pubkey: "Ql96gaTGV7o0QzQbvPBSQfRT5K9PtBNLrxEh9kMV228=", valid: true},
		{mnemonic: "equip will roof matter pink blind book anxiety banner elbow sun young", num: 2, pubkey: "qhbwbWt9k+3SW+68NxXq3knl2jokiGpc11DoKOrjTjo=", valid: true},
		{mnemonic: "equip will roof matter pink blind book anxiety banner elbow sun young", num: 3, pubkey: "skCcMfU7D0olbxZtn8lLn0u41rJzxiCw7rcv3uVEw3Y=", valid: true},
		{mnemonic: "actorr want explain gravity body drill bike update mask wool tell seven", pubkey: "", valid: false},
		{mnemonic: "actor want explain gravity body drill bike update mask wool tell", pubkey: "", valid: false},
		{mnemonic: "", pubkey: "", valid: false},
	}

	for _, m := range mnemonics {
		if m.valid {
			signer, _, err := Sr25519FromMnemonic(m.mnemonic, m.num)
			require.NoError(t, err)
			require.Equal(t, m.pubkey, signer.Public().String())
		} else {
			_, _, err := Sr25519FromMnemonic(m.mnemonic, 0)
			require.Error(t, err)
		}
	}
}
