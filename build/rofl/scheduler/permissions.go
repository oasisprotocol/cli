package scheduler

import (
	"encoding/base64"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

const (
	// MachinePermissionLogView is the permission required to view logs.
	MachinePermissionLogView = "log.view"
)

// Permissions is a map of actions to addresses that are allowed to perform them.
type Permissions map[string][]types.Address

// MarshalPermissions marshals the permissions map into a base64-encoded CBOR string.
func MarshalPermissions(permissions Permissions) string {
	encPerms := cbor.Marshal(permissions)
	return base64.StdEncoding.EncodeToString(encPerms)
}
