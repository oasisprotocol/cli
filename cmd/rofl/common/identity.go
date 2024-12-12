package common

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/sgx"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle/component"

	"github.com/oasisprotocol/cli/build/measurement"
)

// ComputeEnclaveIdentity computes the enclave identity of the given ROFL components. If no specific
// component ID is passed, it uses the first ROFL component.
func ComputeEnclaveIdentity(bnd *bundle.Bundle, compID string) ([]*sgx.EnclaveIdentity, error) {
	var cid component.ID
	if compID != "" {
		if err := cid.UnmarshalText([]byte(compID)); err != nil {
			return nil, fmt.Errorf("malformed component ID: %w", err)
		}
	}

	for _, comp := range bnd.Manifest.GetAvailableComponents() {
		if comp.Kind != component.ROFL {
			continue // Skip non-ROFL components.
		}
		switch compID {
		case "":
			// When not specified we use the first ROFL app.
		default:
			if !comp.Matches(cid) {
				continue
			}
		}

		switch teeKind := comp.TEEKind(); teeKind {
		case component.TEEKindSGX:
			var enclaveID *sgx.EnclaveIdentity
			enclaveID, err := bnd.EnclaveIdentity(comp.ID())
			if err != nil {
				return nil, err
			}
			return []*sgx.EnclaveIdentity{enclaveID}, nil
		case component.TEEKindTDX:
			return measurement.MeasureTdxQemu(bnd, comp)
		default:
			return nil, fmt.Errorf("identity computation for TEE kind '%s' not supported", teeKind)
		}
	}

	switch compID {
	case "":
		return nil, fmt.Errorf("no ROFL apps found in bundle")
	default:
		return nil, fmt.Errorf("ROFL app '%s' not found in bundle", compID)
	}
}
