package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/cli/build/rofl"
)

func TestGetOrcFilename(t *testing.T) {
	require := require.New(t)

	for _, tc := range []struct {
		name       string
		deployment string
		expected   string
	}{
		{"rofl-scheduler", "mainnet", "rofl-scheduler.mainnet.orc"},
		{"ROFL Scheduler", "mainnet", "rofl-scheduler.mainnet.orc"},
		{"   This is a    test   ", "mainnet", "this-is-a-test.mainnet.orc"},
	} {
		manifest := &rofl.Manifest{Name: tc.name}
		fn := GetOrcFilename(manifest, "mainnet")
		require.Equal(tc.expected, fn)
	}
}
