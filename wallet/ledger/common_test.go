package ledger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSerializedPath(t *testing.T) {
	paths := []struct {
		path     []uint32
		expected []byte
		valid    bool
	}{
		{
			path:     []uint32{44, 474, 0}, // adr8, index 0
			expected: []byte{44, 0, 0, 128, 218, 1, 0, 128, 0, 0, 0, 128},
			valid:    true,
		},
		{
			path:     []uint32{44, 474, 1}, // adr8, index 1
			expected: []byte{44, 0, 0, 128, 218, 1, 0, 128, 1, 0, 0, 128},
			valid:    true,
		},
		{
			path:     []uint32{44, 474, 0, 0, 0}, // legacy, index 0
			expected: []byte{44, 0, 0, 128, 218, 1, 0, 128, 0, 0, 0, 128, 0, 0, 0, 128, 0, 0, 0, 128},
			valid:    true,
		},
		{
			path:     []uint32{44, 474, 0, 0, 1}, // legacy, index 1
			expected: []byte{44, 0, 0, 128, 218, 1, 0, 128, 0, 0, 0, 128, 0, 0, 0, 128, 1, 0, 0, 128},
			valid:    true,
		},
		{
			path:     []uint32{}, // wrong length
			expected: []byte{},
			valid:    false,
		},
		{
			path:     []uint32{44}, // wrong length
			expected: []byte{},
			valid:    false,
		},
		{
			path:     []uint32{44, 474, 0, 0}, // wrong length
			expected: []byte{},
			valid:    false,
		},
		{
			path:     []uint32{44, 474, 0, 0, 0, 0}, // wrong length
			expected: []byte{},
			valid:    false,
		},
	}

	for _, p := range paths {
		result, err := getSerializedPath(p.path)
		if p.valid {
			require.Equal(t, p.expected, result)
		} else {
			require.Error(t, err)
		}
	}
}

func TestGetSerializedBip44Path(t *testing.T) {
	paths := []struct {
		path     []uint32
		expected []byte
		valid    bool
	}{
		{
			path:     []uint32{44, 60, 0, 0, 0}, // bip44, change 0, index 0
			expected: []byte{44, 0, 0, 128, 60, 0, 0, 128, 0, 0, 0, 128, 0, 0, 0, 0, 0, 0, 0, 0},
			valid:    true,
		},
		{
			path:     []uint32{44, 60, 0, 0, 1}, // bip44, change 0, index 1
			expected: []byte{44, 0, 0, 128, 60, 0, 0, 128, 0, 0, 0, 128, 0, 0, 0, 0, 1, 0, 0, 0},
			valid:    true,
		},
		{
			path:     []uint32{44, 60, 0, 1, 0}, // bip44, change 1, index 0
			expected: []byte{44, 0, 0, 128, 60, 0, 0, 128, 0, 0, 0, 128, 1, 0, 0, 0, 0, 0, 0, 0},
			valid:    true,
		},
		{
			path:     []uint32{44, 60, 0, 1, 1}, // bip44, change 1, index 1
			expected: []byte{44, 0, 0, 128, 60, 0, 0, 128, 0, 0, 0, 128, 1, 0, 0, 0, 1, 0, 0, 0},
			valid:    true,
		},
		{
			path:     []uint32{}, // wrong length
			expected: []byte{},
			valid:    false,
		},
		{
			path:     []uint32{44}, // wrong length
			expected: []byte{},
			valid:    false,
		},
		{
			path:     []uint32{44, 60, 0}, // wrong length
			expected: []byte{},
			valid:    false,
		},
		{
			path:     []uint32{44, 60, 0, 0}, // wrong length
			expected: []byte{},
			valid:    false,
		},
		{
			path:     []uint32{44, 60, 0, 0, 0, 0}, // wrong length
			expected: []byte{},
			valid:    false,
		},
	}

	for _, p := range paths {
		result, err := getSerializedBip44Path(p.path)
		if p.valid {
			require.Equal(t, p.expected, result)
		} else {
			require.Error(t, err)
		}
	}
}
