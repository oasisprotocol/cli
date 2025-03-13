package common

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/sgx"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"

	"github.com/oasisprotocol/cli/cmd/common"
)

// GetRegisteredEnclaves retrieves currently registered on-chain enclaves for the given deployment.
func GetRegisteredEnclaves(ctx context.Context, rawAppID string, npa *common.NPASelection) (map[sgx.EnclaveIdentity]struct{}, error) {
	var conn connection.Connection
	var err error
	if conn, err = connection.Connect(ctx, npa.Network); err != nil {
		return nil, err
	}

	var appID rofl.AppID
	if err = appID.UnmarshalText([]byte(rawAppID)); err != nil {
		return nil, fmt.Errorf("unable to extract app id: %v", err)
	}

	var appCfg *rofl.AppConfig
	if appCfg, err = conn.Runtime(npa.ParaTime).ROFL.App(ctx, client.RoundLatest, appID); err != nil {
		return nil, err
	}

	cfgEnclaves := make(map[sgx.EnclaveIdentity]struct{})
	for _, eid := range appCfg.Policy.Enclaves {
		cfgEnclaves[eid] = struct{}{}
	}

	return cfgEnclaves, nil
}
