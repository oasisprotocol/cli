package common

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle/component"

	"github.com/oasisprotocol/cli/build/measurement"
)

// ComputeEnclaveIdentity computes the enclave identity of the given ROFL components. If no specific
// component ID is passed, it uses the first ROFL component.
func ComputeEnclaveIdentity(bnd *bundle.Bundle, compID string) ([]bundle.Identity, error) {
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

		return ComputeComponentIdentity(bnd, comp)
	}

	switch compID {
	case "":
		return nil, fmt.Errorf("no ROFL apps found in bundle")
	default:
		return nil, fmt.Errorf("ROFL app '%s' not found in bundle", compID)
	}
}

// ComputeComponentIdentity computes the enclave identity of the given component.
func ComputeComponentIdentity(bnd *bundle.Bundle, comp *bundle.Component) ([]bundle.Identity, error) {
	switch teeKind := comp.TEEKind(); teeKind {
	case component.TEEKindSGX:
		eids, err := bnd.EnclaveIdentities(comp.ID())
		if err != nil {
			return nil, err
		}

		ids := make([]bundle.Identity, len(eids))
		for _, eid := range eids {
			// For SGX enclaves, there is no additional metadata (e.g. hypervisor).
			ids = append(ids, bundle.Identity{Enclave: eid})
		}
		return ids, nil
	case component.TEEKindTDX:
		return measurement.MeasureTdxQemu(bnd, comp)
	default:
		return nil, fmt.Errorf("identity computation for TEE kind '%s' not supported", teeKind)
	}
}
