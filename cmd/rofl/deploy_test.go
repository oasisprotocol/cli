package rofl

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/cli/build/rofl/scheduler"
	roflCmdBuild "github.com/oasisprotocol/cli/cmd/rofl/build"
)

func TestExtractProxyTimeouts(t *testing.T) {
	require.Empty(t, extractProxyTimeouts(nil))
	require.Empty(t, extractProxyTimeouts(&roflCmdBuild.AppExtraConfig{
		Ports: []*roflCmdBuild.PortMapping{{Port: "8080"}},
	}))

	// The maximum across all ports should be requested.
	metadata := extractProxyTimeouts(&roflCmdBuild.AppExtraConfig{
		Ports: []*roflCmdBuild.PortMapping{
			{Port: "80", TimeoutRW: 300, TimeoutConnect: 10},
			{Port: "8080", TimeoutRW: 60, TimeoutConnection: 3600},
			{Port: "9090"},
		},
	})
	require.Equal(t, map[string]string{
		scheduler.MetadataKeyProxyTimeoutRW:         "300",
		scheduler.MetadataKeyProxyTimeoutConnection: "3600",
	}, metadata)
}
