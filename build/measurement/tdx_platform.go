package measurement

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
)

const (
	tdxFirmwareProtocol = "oasis-tdvf-v1"

	tdxHypervisorQEMU101 = "qemu/v10.1"
	tdxHypervisorQEMU82  = "qemu/v8.2"

	maxPlatformCatalogSize = 128 << 20
	profilePCI64Size       = uint64(32 << 30)
)

// Hypervisor IDs name the whole launch contract, not just MRTD ordering.
// qemu/v0 and qemu/v1 are permanently reserved for legacy OVMF.
var tdxProfileSpecs = map[string]struct {
	Machine      string
	QEMUVersion  string
	SourceSHA256 string
	MRTDVariant  int
}{
	tdxHypervisorQEMU101: {"pc-q35-10.1", "10.1.0", "e0517349b50ca73ebec2fa85b06050d5c463ca65c738833bd8fc1f15f180be51", mrtdVariantSinglePass},
	tdxHypervisorQEMU82:  {"pc-q35-8.2", "2:8.2.2+ds-0ubuntu1.4+tdx1.1", "5e692dd20950a819ff6550cd3974e00319a93f47c0a15f808c58df1a9e4d2478", mrtdVariantTwoPass},
}

type platformPatch struct {
	Offset uint32 `json:"offset"`
	Width  uint8  `json:"width"`
	Value  string `json:"value"`
}

type platformTemplate struct {
	Data    []byte          `json:"data"`
	Patches []platformPatch `json:"patches"`
}

type platformACPI struct {
	Tables platformTemplate `json:"tables"`
	RSDP   []byte           `json:"rsdp"`
	Loader []byte           `json:"loader"`
}

type platformFwCfg struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

// tdxPlatformCatalog is the static, firmware-bound oasis-boot release artifact.
// Every supported Hypervisor has its own templates; there is no host probing.
type tdxPlatformCatalog struct {
	Version          uint32                        `json:"version"`
	FirmwareProtocol string                        `json:"firmware_protocol"`
	FirmwareSHA256   string                        `json:"firmware_sha256"`
	MaxMemoryMiB     uint64                        `json:"max_memory_mib"`
	MaxCPUs          uint16                        `json:"max_cpus"`
	Profiles         map[string]tdxPlatformProfile `json:"profiles"`
}

type tdxPlatformProfile struct {
	Machine      string                      `json:"machine"`
	QEMUVersion  string                      `json:"qemu_version"`
	SourceSHA256 string                      `json:"source_sha256"`
	ACPI         map[string]platformACPI     `json:"acpi"`
	SMBIOS       map[string]platformTemplate `json:"smbios"`
	FwCfg        []platformFwCfg             `json:"fw_cfg"`
}

// TdxPlatformInputs contains the bytes the pinned firmware measures. It is
// derived entirely offline from the release profile and normalized launch config.
type TdxPlatformInputs struct {
	Hypervisor string          `json:"hypervisor"`
	TDHOB      []byte          `json:"td_hob"`
	ACPI       []byte          `json:"acpi"`
	RSDP       []byte          `json:"rsdp"`
	Loader     []byte          `json:"loader"`
	SMBIOS     []byte          `json:"smbios"`
	FwCfg      []platformFwCfg `json:"fw_cfg"`
}

func decodeStrictJSON(raw []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func readTdxPlatformCatalog(raw, fw []byte) (*tdxPlatformCatalog, error) {
	if len(raw) > maxPlatformCatalogSize {
		return nil, fmt.Errorf("TDX platform catalog is too large")
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("TDX platform catalog gzip: %w", err)
	}
	defer zr.Close()
	data, err := io.ReadAll(io.LimitReader(zr, maxPlatformCatalogSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPlatformCatalogSize {
		return nil, fmt.Errorf("TDX platform catalog expands beyond size limit")
	}

	var p tdxPlatformCatalog
	if err = decodeStrictJSON(data, &p); err != nil {
		return nil, fmt.Errorf("TDX platform catalog JSON: %w", err)
	}
	if p.Version != 2 || p.FirmwareProtocol != tdxFirmwareProtocol ||
		p.MaxCPUs != 255 || p.MaxMemoryMiB != 1048576 {
		return nil, fmt.Errorf("unsupported TDX platform catalog")
	}
	if p.FirmwareSHA256 != fmt.Sprintf("%x", sha256.Sum256(fw)) {
		return nil, fmt.Errorf("TDX platform catalog does not match firmware SHA-256")
	}

	for hypervisor, spec := range tdxProfileSpecs {
		profile, ok := p.Profiles[hypervisor]
		if !ok || profile.Machine != spec.Machine || profile.QEMUVersion != spec.QEMUVersion ||
			profile.SourceSHA256 != spec.SourceSHA256 {
			return nil, fmt.Errorf("unsupported or missing TDX platform profile %s", hypervisor)
		}
	}
	return &p, nil
}

func (t *platformTemplate) expand(memory uint64) ([]byte, error) {
	low, high := tdxQemuRAMSplit(memory)
	// The explicit OVMF override disables its host-width-dependent dynamic
	// aperture. Naturally align the fixed 32 GiB window above high RAM.
	pci64Base := ((1 << 32) + high + profilePCI64Size - 1) & ^(profilePCI64Size - 1)
	values := map[string]uint64{
		"low_ram":       low,
		"pci32_size":    0xe0000000 - low,
		"pci64_base":    pci64Base,
		"pci64_end":     pci64Base + profilePCI64Size - 1,
		"memory_kib":    memory * 1024,
		"dimm_mib":      min(memory, 32767),
		"last_dimm_mib": (memory-1)%16384 + 1,
		"low_end_kib":   low/1024 - 1,
		"high_end_kib":  ((1<<32)+high)/1024 - 1,
	}
	if len(t.Data) == 0 || len(t.Data) > 256<<10 || len(t.Patches) > 16 {
		return nil, fmt.Errorf("invalid platform template size")
	}
	if memory >= 32767 {
		values["extended_dimm_mib"] = memory
	} else {
		values["extended_dimm_mib"] = 0
	}

	out := bytes.Clone(t.Data)
	used := make(map[uint32]bool)
	for _, p := range t.Patches {
		value, ok := values[p.Value]
		if !ok || (p.Width != 2 && p.Width != 4 && p.Width != 8) ||
			uint64(p.Offset)+uint64(p.Width) > uint64(len(out)) {
			return nil, fmt.Errorf("invalid platform template relocation")
		}
		if p.Width < 8 && value >= uint64(1)<<(8*p.Width) {
			return nil, fmt.Errorf("platform relocation overflows")
		}
		for i := uint32(0); i < uint32(p.Width); i++ {
			off := p.Offset + i
			if used[off] || out[off] != 0 {
				return nil, fmt.Errorf("overlapping or nonzero platform relocation")
			}
			used[off] = true
			out[off] = byte((value >> (8 * i)) & 0xff)
		}
	}
	return out, nil
}

func generateTdxPlatformInputs(bnd *bundle.Bundle, comp *bundle.Component, fw []byte, tdvfMeta *tdvfMetadata) ([]*TdxPlatformInputs, error) {
	if comp.TDX == nil {
		return nil, fmt.Errorf("component does not support TDX")
	}

	platformData, ok := bnd.Data[comp.TDX.Platform]
	if !ok {
		return nil, fmt.Errorf("missing TDX platform catalog")
	}
	platform, err := bundle.ReadAllData(platformData)
	if err != nil {
		return nil, err
	}

	p, err := readTdxPlatformCatalog(platform, fw)
	if err != nil {
		return nil, err
	}
	hob, err := tdxQemuTdHob(comp.TDX.Resources.Memory, tdvfMeta)
	if err != nil {
		return nil, err
	}

	hypervisors := []string{
		tdxHypervisorQEMU101,
		tdxHypervisorQEMU82,
	}
	inputs := make([]*TdxPlatformInputs, 0, len(hypervisors))
	for _, hypervisor := range hypervisors {
		profile := p.Profiles[hypervisor]
		input, err := profile.generate(hypervisor, comp.TDX, hob)
		if err != nil {
			return nil, fmt.Errorf("generate %s platform: %w", hypervisor, err)
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}

func (p *tdxPlatformProfile) generate(hypervisor string, cfg *bundle.TDXMetadata, hob []byte) (*TdxPlatformInputs, error) {
	a, ok := p.ACPI[strconv.Itoa(int(cfg.Resources.CPUCount))]
	if !ok {
		return nil, fmt.Errorf("missing ACPI template for %d vCPUs", cfg.Resources.CPUCount)
	}
	if len(a.RSDP) != 20 || len(a.Loader) != 4096 {
		return nil, fmt.Errorf("invalid ACPI loader or RSDP size")
	}

	inputs := &TdxPlatformInputs{
		Hypervisor: hypervisor,
		TDHOB:      bytes.Clone(hob),
		RSDP:       bytes.Clone(a.RSDP),
		Loader:     bytes.Clone(a.Loader),
		FwCfg:      p.FwCfg,
	}

	var err error
	if inputs.ACPI, err = a.Tables.expand(cfg.Resources.Memory); err != nil {
		return nil, err
	}
	key := "low"
	if cfg.Resources.Memory >= 2816 {
		key = "high"
	}
	if hypervisor == tdxHypervisorQEMU82 {
		key += "-" + strconv.FormatUint((cfg.Resources.Memory+16383)/16384, 10)
	}
	tpl, ok := p.SMBIOS[key]
	if !ok {
		return nil, fmt.Errorf("missing SMBIOS template for memory configuration")
	}
	if inputs.SMBIOS, err = tpl.expand(cfg.Resources.Memory); err != nil {
		return nil, err
	}
	return inputs, nil
}
