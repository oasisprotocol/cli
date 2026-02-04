package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeOmitEmptyMapstructure(t *testing.T) {
	require := require.New(t)

	t.Run("omitempty omits empty string field", func(_ *testing.T) {
		entry := AddressBookEntry{
			Description: "d",
			Address:     "oasis1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39y6h",
			EthAddress:  "",
		}

		enc, err := encode(entry)
		require.NoError(err)
		m, ok := enc.(map[string]interface{})
		require.True(ok)
		require.NotContains(m, "eth_address")
	})

	t.Run("omitempty includes non-empty string field", func(_ *testing.T) {
		entry := AddressBookEntry{
			Address:    "oasis1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39y6h",
			EthAddress: "0x60a6321ea71d37102dbf923aae2e08d005c4e403",
		}

		enc, err := encode(entry)
		require.NoError(err)
		m, ok := enc.(map[string]interface{})
		require.True(ok)
		require.Equal(entry.EthAddress, m["eth_address"])
	})
}
