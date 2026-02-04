package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

func TestPrettyAddressWith(t *testing.T) {
	require := require.New(t)

	nativeAddr, ethAddr, err := helpers.ResolveEthOrOasisAddress("0x60a6321eA71d37102Dbf923AAe2E08d005C4e403")
	require.NoError(err)
	require.NotNil(nativeAddr)
	require.NotNil(ethAddr)

	t.Run("eth preferred when known", func(_ *testing.T) {
		ctx := AddressFormatContext{
			Names: types.AccountNames{
				nativeAddr.String(): "my",
			},
			Eth: map[string]string{
				nativeAddr.String(): ethAddr.Hex(),
			},
		}

		require.Equal("my ("+ethAddr.Hex()+")", PrettyAddressWith(ctx, nativeAddr.String()))
		require.Equal("my ("+ethAddr.Hex()+")", PrettyAddressWith(ctx, ethAddr.Hex()))
	})

	t.Run("native fallback when eth unknown", func(_ *testing.T) {
		ctx := AddressFormatContext{
			Names: types.AccountNames{
				nativeAddr.String(): "my",
			},
			Eth: map[string]string{},
		}

		require.Equal("my ("+nativeAddr.String()+")", PrettyAddressWith(ctx, nativeAddr.String()))
		// If the user explicitly provided an Ethereum address, prefer it even if not in ctx.Eth.
		require.Equal("my ("+ethAddr.Hex()+")", PrettyAddressWith(ctx, ethAddr.Hex()))
	})

	t.Run("unknown returns unchanged", func(_ *testing.T) {
		ctx := AddressFormatContext{
			Names: types.AccountNames{},
			Eth:   map[string]string{},
		}

		require.Equal(nativeAddr.String(), PrettyAddressWith(ctx, nativeAddr.String()))
		require.Equal(ethAddr.Hex(), PrettyAddressWith(ctx, ethAddr.Hex()))
	})

	t.Run("unparseable returns unchanged", func(_ *testing.T) {
		ctx := AddressFormatContext{
			Names: types.AccountNames{
				nativeAddr.String(): "my",
			},
			Eth: map[string]string{
				nativeAddr.String(): ethAddr.Hex(),
			},
		}

		require.Equal("not-an-address", PrettyAddressWith(ctx, "not-an-address"))
	})
}
