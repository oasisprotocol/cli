package file //nolint: dupl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEd25519FromMnemonic(t *testing.T) {
	mnemonics := []struct {
		mnemonic string
		num      uint32
		pubkey   string
		valid    bool
	}{
		{mnemonic: "equip will roof matter pink blind book anxiety banner elbow sun young", num: 0, pubkey: "RWAfdhrxfbpQJDUp5ilzLxxY0I/92qhJEjhUBHVynYU=", valid: true},
		{mnemonic: "equip will roof matter pink blind book anxiety banner elbow sun young", num: 1, pubkey: "J+0Eo8Dc7GWRwAHk6jB9ZcvXEsuQ2Fq3cDw17uB6d90=", valid: true},
		{mnemonic: "equip will roof matter pink blind book anxiety banner elbow sun young", num: 2, pubkey: "GUVqPwzz9MxebOUt71fZK7PFplH6liayRs/sB6vChyQ=", valid: true},
		{mnemonic: "equip will roof matter pink blind book anxiety banner elbow sun young", num: 3, pubkey: "klSQRiFP20cpv3pu5KO70PRjxHasyTOyx8zghFCavuQ=", valid: true},
		{mnemonic: "actorr want explain gravity body drill bike update mask wool tell seven", pubkey: "", valid: false},
		{mnemonic: "actor want explain gravity body drill bike update mask wool tell", pubkey: "", valid: false},
		{mnemonic: "", pubkey: "", valid: false},
	}

	for _, m := range mnemonics {
		if m.valid {
			signer, _, err := Ed25519FromMnemonic(m.mnemonic, m.num)
			require.NoError(t, err)
			require.Equal(t, m.pubkey, signer.Public().String())
		} else {
			_, _, err := Ed25519FromMnemonic(m.mnemonic, 0)
			require.Error(t, err)
		}
	}
}
