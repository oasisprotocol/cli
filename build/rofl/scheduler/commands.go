package scheduler

import (
	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
)

// Commands supported by the scheduler.
const (
	MethodDeploy    = "Deploy"
	MethodRestart   = "Restart"
	MethodTerminate = "Terminate"
)

// Command is a command to be executed on a specific instance by the scheduler.
type Command struct {
	// Method is the method name.
	Method string `json:"method"`
	// Args are the method arguments.
	Args cbor.RawMessage `json:"args"`
}

// DeployRequest is a deployment request.
type DeployRequest struct {
	// Deployment is the deployment to deploy.
	Deployment roflmarket.Deployment `json:"deployment"`
	// WipeStorage is a flag indicating whether persistent storage should be wiped.
	WipeStorage bool `json:"wipe_storage"`
}

// RestartRequest is an instance restart request.
type RestartRequest struct {
	// WipeStorage is a flag indicating whether persistent storage should be wiped.
	WipeStorage bool `json:"wipe_storage"`
}

// TerminateRequest is an instance termination request.
type TerminateRequest struct {
	// WipeStorage is a flag indicating whether persistent storage should be wiped.
	WipeStorage bool `json:"wipe_storage"`
}
