package measurement

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
)

const (
	tdxFirmwareProtocol = "oasis-tdvf-v1"

	tdxHypervisorQEMU101 = "qemu/v10.1"
	tdxHypervisorQEMU82  = "qemu/v8.2"

	maxPlatformCatalogSize = 128 << 20
	profilePCI64Size       = uint64(32 << 30)

	// All published oasis-boot releases from v0.1.0 through v0.6.3 contain this
	// same firmware. Recognize legacy artifacts by bytes, not missing metadata.
	legacyTdxFirmwareSHA256 = "db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f"
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

func isLegacyTdxFirmware(fw []byte) bool {
	return fmt.Sprintf("%x", sha256.Sum256(fw)) == legacyTdxFirmwareSHA256
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

// TODO: This should be removed.
func TdxProfileFilenames(comp *bundle.Component) string {
	// TODO: This should be comp.TDX.Platform once core is updated.
	return "platform.json.gz"
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
	if isLegacyTdxFirmware(fw) {
		return nil, fmt.Errorf("legacy firmware does not support %s", tdxFirmwareProtocol)
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

// profileRAM preserves Q35's legacy split with max-ram-below-4g=4G.
func profileRAM(memory uint64) (low, high uint64) {
	if memory < 2816 {
		return memory << 20, 0
	}
	return 2048 << 20, (memory - 2048) << 20
}

func (t platformTemplate) expand(memory uint64) ([]byte, error) {
	low, high := profileRAM(memory)
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

// GenerateTdxPlatformInputs reproduces the measured bytes for every permitted
// hypervisor, in canonical order.
func GenerateTdxPlatformInputs(bnd *bundle.Bundle, comp *bundle.Component) ([]TdxPlatformInputs, error) {
	if comp.TDX == nil {
		return nil, fmt.Errorf("component does not support TDX")
	}

	fwData, ok := bnd.Data[comp.TDX.Firmware]
	if !ok {
		return nil, fmt.Errorf("missing firmware")
	}
	fw, err := bundle.ReadAllData(fwData)
	if err != nil {
		return nil, err
	}

	profileName := TdxProfileFilenames(comp)
	read := func(name string, limit int64) ([]byte, error) {
		data, exists := bnd.Data[name]
		if !exists {
			return nil, fmt.Errorf("missing TDX bundle asset %s", name)
		}
		r, e := data.Open()
		if e != nil {
			return nil, e
		}
		defer r.Close()
		raw, e := io.ReadAll(io.LimitReader(r, limit+1))
		if int64(len(raw)) > limit {
			return nil, fmt.Errorf("TDX asset %s is too large", name)
		}
		return raw, e
	}
	raw, err := read(profileName, maxPlatformCatalogSize)
	if err != nil {
		return nil, err
	}
	p, err := readTdxPlatformCatalog(raw, fw)
	if err != nil {
		return nil, err
	}

	meta, err := parseTdvfMetadata(fw)
	if err != nil {
		return nil, err
	}
	hob, err := generateProfileHOB(comp.TDX.Resources.Memory, meta)
	if err != nil {
		return nil, err
	}

	// TODO: Make this a constant.
	hypervisors := []string{tdxHypervisorQEMU101, tdxHypervisorQEMU82}
	inputs := make([]TdxPlatformInputs, 0, len(hypervisors))
	for _, hypervisor := range hypervisors {
		input, err := p.Profiles[hypervisor].generate(hypervisor, comp.TDX, hob)
		if err != nil {
			return nil, fmt.Errorf("generate %s platform: %w", hypervisor, err)
		}
		inputs = append(inputs, *input)
	}
	return inputs, nil
}

func (p tdxPlatformProfile) generate(hypervisor string, cfg *bundle.TDXMetadata, hob []byte) (*TdxPlatformInputs, error) {
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

// generateProfileHOB mirrors both pinned QEMUs' tdx_accept_ram_range and tdvf_hob_create.
// Preserve descriptor boundaries: adjacent accepted TDVF sections are not merged.
// OVMF hashes the handoff/resource HOBs, excluding the terminating 8-byte HOB.
func generateProfileHOB(memory uint64, meta *tdvfMetadata) ([]byte, error) {
	type region struct {
		start, size uint64
		kind        uint32
	}
	low, high := profileRAM(memory)
	regions := []region{{0, low, 7}}
	if high != 0 {
		regions = append(regions, region{1 << 32, high, 7})
	}
	var hobBase, hobSize uint64
	for _, s := range meta.sections {
		if s.secType > 3 || s.attributes&attributePageAug != 0 {
			return nil, fmt.Errorf("unsupported profile TDVF section")
		}
		if s.secType != 2 && s.secType != 3 {
			continue
		}
		if s.secType == 2 {
			if hobSize != 0 {
				return nil, fmt.Errorf("duplicate TD HOB section")
			}
			hobBase, hobSize = s.memoryAddress, s.memoryDataSize
		}
		found := false
		for i, r := range regions {
			if r.kind != 7 || s.memoryAddress < r.start || s.memoryAddress+s.memoryDataSize > r.start+r.size {
				continue
			}
			regions[i] = region{s.memoryAddress, s.memoryDataSize, 0}
			if s.memoryAddress > r.start {
				regions = append(regions, region{r.start, s.memoryAddress - r.start, 7})
			}
			end := s.memoryAddress + s.memoryDataSize
			if end < r.start+r.size {
				regions = append(regions, region{end, r.start + r.size - end, 7})
			}
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("TDVF temporary memory is outside RAM or overlaps")
		}
	}
	slices.SortFunc(regions, func(a, b region) int {
		if a.start < b.start {
			return -1
		}
		if a.start > b.start {
			return 1
		}
		return 0
	})
	length := 56 + 48*len(regions)
	if hobSize == 0 || uint64(length+8) > hobSize {
		return nil, fmt.Errorf("TD HOB section too small or missing")
	}
	out := make([]byte, length)
	binary.LittleEndian.PutUint16(out, 1)
	binary.LittleEndian.PutUint16(out[2:], 56)
	binary.LittleEndian.PutUint32(out[8:], 9)
	binary.LittleEndian.PutUint64(out[48:], hobBase+uint64(length)+8)
	for i, r := range regions {
		d := out[56+i*48:]
		binary.LittleEndian.PutUint16(d, 3)
		binary.LittleEndian.PutUint16(d[2:], 48)
		binary.LittleEndian.PutUint32(d[24:], r.kind)
		binary.LittleEndian.PutUint32(d[28:], 7)
		binary.LittleEndian.PutUint64(d[32:], r.start)
		binary.LittleEndian.PutUint64(d[40:], r.size)
	}
	return out, nil
}
