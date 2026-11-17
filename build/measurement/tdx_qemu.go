package measurement

import (
	"bytes"
	"crypto"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/foxboron/go-uefi/authenticode"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/oasisprotocol/oasis-core/go/common/sgx/pcs"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"

	"github.com/oasisprotocol/cli/build/measurement/acpi"
)

// measureSha384 computes a SHA384 of the given blob.
func measureSha384(data []byte) []byte {
	h := sha512.Sum384(data)
	return h[:]
}

// measureTdxKernelCmdline measures the kernel cmdline.
func measureTdxKernelCmdline(cmdline string) []byte {
	// Add a NUL byte at the end.
	d := append([]byte(cmdline), 0x00)
	// Convert to UTF-16LE.
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	xr := transform.NewReader(bytes.NewReader(d), utf16le)
	converted, _ := io.ReadAll(xr)
	return measureSha384(converted)
}

// tdxQemuRAMSplit preserves Q35's legacy split with max-ram-below-4g=4G.
func tdxQemuRAMSplit(memory uint64) (low, high uint64) {
	if memory < 2816 {
		return memory << 20, 0
	}
	return 2048 << 20, (memory - 2048) << 20
}

// measureTdxQemuTdHob measures the TD HOB.
func measureTdxQemuTdHob(resources *bundle.TDXResources, meta *tdvfMetadata) ([]byte, error) {
	tdHob, err := tdxQemuTdHob(resources.Memory, meta)
	if err != nil {
		return nil, err
	}
	return measureSha384(tdHob), nil
}

// tdxQemuTdHob mirrors both pinned QEMUs' tdx_accept_ram_range and tdvf_hob_create.
// Preserve descriptor boundaries: adjacent accepted TDVF sections are not merged.
// OVMF hashes the handoff/resource HOBs, excluding the terminating 8-byte HOB.
func tdxQemuTdHob(memory uint64, tdvfMeta *tdvfMetadata) ([]byte, error) {
	// Construct the region list from the TDVF metadata.
	type region struct {
		start, size uint64
		kind        uint32
	}
	low, high := tdxQemuRAMSplit(memory)
	regions := []region{{0, low, 7}}
	if high != 0 {
		regions = append(regions, region{1 << 32, high, 7})
	}
	var hobBase, hobSize uint64
	for _, s := range tdvfMeta.Sections {
		if s.SecType > 3 || s.Attributes&attributePageAug != 0 {
			return nil, fmt.Errorf("unsupported profile TDVF section")
		}
		if s.SecType != tdvfSectionTdHob && s.SecType != tdvfSectionTempMem {
			continue
		}

		// Discover the TD HOB base address from TDVF metadata.
		if s.SecType == tdvfSectionTdHob {
			if hobSize != 0 {
				return nil, fmt.Errorf("duplicate TD HOB section")
			}
			hobBase, hobSize = s.MemoryAddress, s.MemoryDataSize
		}

		found := false
		for i, r := range regions {
			if r.kind != 7 || s.MemoryAddress < r.start || s.MemoryAddress+s.MemoryDataSize > r.start+r.size {
				continue
			}
			regions[i] = region{s.MemoryAddress, s.MemoryDataSize, 0}
			if s.MemoryAddress > r.start {
				regions = append(regions, region{r.start, s.MemoryAddress - r.start, 7})
			}
			end := s.MemoryAddress + s.MemoryDataSize
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

	// Construct a TD hob in the same way as QEMU does.
	// Start with EFI_HOB_TYPE_HANDOFF.
	tdHob := make([]byte, length)
	binary.LittleEndian.PutUint16(tdHob, 1)                             // Header.HobType (EFI_HOB_TYPE_HANDOFF)
	binary.LittleEndian.PutUint16(tdHob[2:], 56)                        // Header.HobLength (56 bytes)
	binary.LittleEndian.PutUint32(tdHob[8:], 9)                         // Version (EFI_HOB_HANDOFF_TABLE_VERSION)
	binary.LittleEndian.PutUint64(tdHob[48:], hobBase+uint64(length)+8) // EfiEndOfHobList

	// The rest of the HOBs are EFI_HOB_TYPE_RESOURCE_DESCRIPTOR.
	for i, r := range regions {
		d := tdHob[56+i*48:]
		binary.LittleEndian.PutUint16(d, 3)            // Header.HobType (EFI_HOB_TYPE_RESOURCE_DESCRIPTOR)
		binary.LittleEndian.PutUint16(d[2:], 48)       // Header.HobLength (48 bytes)
		binary.LittleEndian.PutUint32(d[24:], r.kind)  // ResourceType
		binary.LittleEndian.PutUint32(d[28:], 7)       // ResourceAttribute
		binary.LittleEndian.PutUint64(d[32:], r.start) // PhysicalStart
		binary.LittleEndian.PutUint64(d[40:], r.size)  // Length
	}
	return tdHob, nil
}

// measureLog computes a measurement of the given RTMR event log by simulating extending the RTMR.
func measureLog(log [][]byte) []byte {
	var mr [48]byte // Initialize to zero.
	for _, entry := range log {
		h := sha512.New384()
		_, _ = h.Write(mr[:])
		_, _ = h.Write(entry)
		copy(mr[:], h.Sum([]byte{}))
	}
	return mr[:]
}

// measureTdxQemuAcpiTablesLegacy measures QEMU-generated ACPI tables for TDX.
func measureTdxQemuAcpiTablesLegacy(resources *bundle.TDXResources) ([]byte, []byte, []byte, error) {
	// Generate ACPI tables.
	acpiTables, acpiRsdp, acpiLoader, err := acpi.GenerateTablesQemu(resources)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate ACPI tables: %w", err)
	}

	return measureSha384(acpiTables), measureSha384(acpiRsdp), measureSha384(acpiLoader), nil
}

// measureTdxQemuKernelImage measures QEMU-patched TDX kernel image.
func measureTdxQemuKernelImage(bnd *bundle.Bundle, comp *bundle.Component) ([]byte, error) {
	data, ok := bnd.Data[comp.TDX.Kernel]
	if !ok {
		return nil, fmt.Errorf("missing kernel image in bundle")
	}

	kd, err := bundle.ReadAllData(data)
	if err != nil {
		return nil, err
	}

	if len(kd) < 556 {
		return nil, fmt.Errorf("truncated kernel image")
	}
	kd = bytes.Clone(kd)

	// Qemu performs the following modifications to the linux kernel header.
	//
	// See the x86_load_linux function in hw/i386/x86.c in the qemu repository for the updates and
	// the Linux boot protocol: https://www.kernel.org/doc/html/latest/arch/x86/boot.html.
	//
	// We assume the kernel and the boot protocol version is recent.
	realAddr := uint32(0x10000)
	cmdlineAddr := uint32(0x20000)

	kd[0x210] = 0xb0                                                             // type_of_loader = Qemu v0
	kd[0x211] |= 0x80                                                            // loadflags |= CAN_USE_HEAP
	binary.LittleEndian.PutUint32(kd[0x228:0x228+4], cmdlineAddr)                // cmd_line_ptr
	binary.LittleEndian.PutUint32(kd[0x224:0x224+4], cmdlineAddr-realAddr-0x200) // heap_end_ptr

	parsed, err := authenticode.Parse(bytes.NewReader(kd))
	if err != nil {
		return nil, err
	}
	return parsed.Hash(crypto.SHA384), nil
}

// encodeGUID encodes an UEFI GUID into binary form.
func encodeGUID(guid string) []byte {
	var data []byte
	atoms := strings.Split(guid, "-")
	for idx, atom := range atoms {
		raw, err := hex.DecodeString(atom)
		if err != nil {
			panic("bad GUID")
		}

		if idx <= 2 {
			// Little-endian.
			for i := range raw {
				data = append(data, raw[len(raw)-1-i])
			}
		} else {
			// Big-endian.
			data = append(data, raw...)
		}
	}
	return data
}

const (
	efiVariableGlobal        = "8be4df61-93ca-11d2-aa0d-00e098032b8c"
	efiVariableImageSecurity = "d719b2cb-3d3a-4596-a3bc-dad00e67656f"
)

// measureEfiVariableValue measures an EFI variable event.
func measureEfiVariableValue(guid, name string, value []byte) []byte {
	data := append([]byte{}, encodeGUID(guid)...)
	encoded := utf16.Encode([]rune(name))

	data = binary.LittleEndian.AppendUint64(data, uint64(len(encoded)))
	data = binary.LittleEndian.AppendUint64(data, uint64(len(value)))

	for _, c := range encoded {
		data = binary.LittleEndian.AppendUint16(data, c)
	}

	return measureSha384(append(data, value...))
}

const (
	attributeMrExtend   = 0b00000000_00000000_00000000_00000001
	attributePageAug    = 0b00000000_00000000_00000000_00000010
	pageSize            = 0x1000
	mrExtendGranularity = 0x100

	tdvfSectionCFV     = 0x01
	tdvfSectionTdHob   = 0x02
	tdvfSectionTempMem = 0x03
)

type tdvfSection struct {
	DataOffset     uint32
	RawDataSize    uint32
	MemoryAddress  uint64
	MemoryDataSize uint64
	SecType        uint32
	Attributes     uint32
}

type tdvfMetadata struct {
	Sections []*tdvfSection
}

const (
	mrtdVariantTwoPass    = 0
	mrtdVariantSinglePass = 1
)

func (m *tdvfMetadata) computeMrtd(fw []byte, variant int) []byte {
	h := sha512.New384()

	memPageAdd := func(s *tdvfSection, page uint64) {
		if s.Attributes&attributePageAug == 0 {
			// Use TDCALL [TDH.MEM.PAGE.ADD].
			//
			// Byte 0 through 11 contain the ASCII string 'MEM.PAGE.ADD'.
			// Byte 16 through 23 contain the GPA (in little-endian format).
			// All the other bytes contain 0.
			var buf [128]byte
			copy(buf[:12], []byte("MEM.PAGE.ADD"))
			binary.LittleEndian.PutUint64(buf[16:24], s.MemoryAddress+page*pageSize)
			_, _ = h.Write(buf[:])
		}
	}

	mrExtend := func(s *tdvfSection, page uint64) {
		if s.Attributes&attributeMrExtend != 0 {
			// Need TDCALL [TDH.MR.EXTEND].
			for i := range pageSize / mrExtendGranularity {
				// Byte 0 through 8 contain the ASCII string 'MR.EXTEND'.
				// Byte 16 through 23 contain the GPA (in little-endian format).
				// All the other bytes contain 0.
				var buf [128]byte
				copy(buf[:9], []byte("MR.EXTEND"))
				binary.LittleEndian.PutUint64(buf[16:24], s.MemoryAddress+page*pageSize+uint64(i*mrExtendGranularity))
				_, _ = h.Write(buf[:])

				// The other two extension buffers contain the chunk’s content.
				chunkOffset := int(s.DataOffset) + int(page*pageSize) + i*mrExtendGranularity //nolint: gosec
				_, _ = h.Write(fw[chunkOffset : chunkOffset+mrExtendGranularity])
			}
		}
	}

	for _, s := range m.Sections {
		numPages := s.MemoryDataSize / pageSize

		// There are two known implementations of how QEMU is performing TD initialization:
		//
		// - First add all pages using MEM.PAGE.ADD and then in a second pass perform MR.EXTEND for
		//   for each page (Variant 0).
		//
		// - For each page first add it using MEM.PAGE.ADD and then perform MR.EXTEND for that same
		//   page (Variant 1).
		//
		// Unfortunately, changing these orders changes the MRTD computation so we need both.
		switch variant {
		case mrtdVariantTwoPass:
			for page := range numPages {
				memPageAdd(s, page)
			}
			for page := range numPages {
				mrExtend(s, page)
			}
		case mrtdVariantSinglePass:
			for page := range numPages {
				memPageAdd(s, page)
				mrExtend(s, page)
			}
		default:
			panic("unknown MRTD variant")
		}
	}
	return h.Sum(nil)
}

// ovmfTableEntry walks the bounded OVMF GUID footer table. Lengths include the
// length/GUID trailer (18 bytes), including the footer's own trailer.
func ovmfTableEntry(fw []byte, wanted string) ([]byte, error) {
	if len(fw) < 50 {
		return nil, fmt.Errorf("truncated firmware footer")
	}
	end := len(fw) - 32
	if !bytes.Equal(fw[end-16:end], encodeGUID("96b582de-1fb2-45f7-baea-a366c55a082d")) {
		return nil, fmt.Errorf("malformed OVMF table footer")
	}
	size := int(binary.LittleEndian.Uint16(fw[end-18 : end-16]))
	if size < 18 || size > end {
		return nil, fmt.Errorf("invalid OVMF footer size")
	}
	start := end - size
	var result []byte
	guid := encodeGUID(wanted)
	for end -= 18; end > start; {
		if end-start < 18 {
			return nil, fmt.Errorf("truncated OVMF footer entry")
		}
		n := int(binary.LittleEndian.Uint16(fw[end-18 : end-16]))
		if n < 18 || n > end-start {
			return nil, fmt.Errorf("invalid OVMF footer entry size")
		}
		if bytes.Equal(fw[end-16:end], guid) {
			if result != nil {
				return nil, fmt.Errorf("duplicate OVMF footer entry")
			}
			result = fw[end-n : end-18]
		}
		end -= n
	}
	return result, nil
}

// parseTdvfMetadata parses the TDVF metadata from the firmware blob.
//
// See Section 11 of "Intel TDX Virtual Firmware Design Guide" for details.
func parseTdvfMetadata(fw []byte) (*tdvfMetadata, error) {
	data, err := ovmfTableEntry(fw, "e47a6535-984a-4798-865e-4685a7bf8ec2")
	if err != nil {
		return nil, err
	}
	if len(data) != 4 {
		return nil, fmt.Errorf("missing or invalid TDVF metadata offset")
	}
	distance := uint64(binary.LittleEndian.Uint32(data))
	if distance < 16 || distance > uint64(len(fw)) {
		return nil, fmt.Errorf("TDVF metadata offset out of bounds")
	}
	offset := uint64(len(fw)) - distance
	desc := fw[offset : offset+16]
	if string(desc[:4]) != "TDVF" || binary.LittleEndian.Uint32(desc[8:]) != 1 {
		return nil, fmt.Errorf("unsupported TDVF metadata descriptor")
	}
	size := uint64(binary.LittleEndian.Uint32(desc[4:]))
	count := uint64(binary.LittleEndian.Uint32(desc[12:]))
	if count == 0 || count > 64 || size != 16+32*count || size > distance {
		return nil, fmt.Errorf("invalid TDVF metadata section count/size")
	}

	var meta tdvfMetadata
	var total uint64
	for i := uint64(0); i < count; i++ {
		d := fw[offset+16+32*i : offset+16+32*(i+1)]
		sec := &tdvfSection{
			DataOffset:     binary.LittleEndian.Uint32(d),
			RawDataSize:    binary.LittleEndian.Uint32(d[4:]),
			MemoryAddress:  binary.LittleEndian.Uint64(d[8:]),
			MemoryDataSize: binary.LittleEndian.Uint64(d[16:]),
			SecType:        binary.LittleEndian.Uint32(d[24:]),
			Attributes:     binary.LittleEndian.Uint32(d[28:]),
		}

		// Validate section.
		if sec.MemoryAddress%pageSize != 0 || sec.MemoryDataSize == 0 || sec.MemoryDataSize%pageSize != 0 ||
			sec.MemoryAddress+sec.MemoryDataSize < sec.MemoryAddress ||
			uint64(sec.DataOffset)+uint64(sec.RawDataSize) > uint64(len(fw)) ||
			uint64(sec.RawDataSize) > sec.MemoryDataSize || sec.Attributes & ^uint32(3) != 0 ||
			(sec.Attributes&attributeMrExtend != 0 && uint64(sec.RawDataSize) != sec.MemoryDataSize) {
			return nil, fmt.Errorf("invalid TDVF section %d", i)
		}

		// Validate total memory size.
		total += sec.MemoryDataSize
		if total > 64<<20 {
			return nil, fmt.Errorf("TDVF memory size exceeds limit")
		}

		// Check for overlapping sections.
		for _, prev := range meta.Sections {
			if sec.MemoryAddress < prev.MemoryAddress+prev.MemoryDataSize && prev.MemoryAddress < sec.MemoryAddress+sec.MemoryDataSize {
				return nil, fmt.Errorf("overlapping TDVF memory sections")
			}
		}
		meta.Sections = append(meta.Sections, sec)
	}
	return &meta, nil
}

// MeasureTdxQemu computes the TD measurements for the given component. It assumes that a known
// virtual firmware image is used that follows the measurement protocol and that QEMU is used as the
// hypervisor.
//
// It may return multiple identities because there may be differences between QEMU versions that can
// cause differences in measurements (e.g. with MRTD).
func MeasureTdxQemu(bnd *bundle.Bundle, comp *bundle.Component) ([]bundle.Identity, error) {
	if comp.TDX == nil {
		return nil, fmt.Errorf("component does not support TDX")
	}
	fwData, ok := bnd.Data[comp.TDX.Firmware]
	if !ok {
		return nil, fmt.Errorf("missing firmware image in bundle")
	}

	fw, err := bundle.ReadAllData(fwData)
	if err != nil {
		return nil, err
	}

	// Parse TDVF metadata.
	tdvfMeta, err := parseTdvfMetadata(fw)
	if err != nil {
		return nil, err
	}

	if !comp.TDX.HasPlatform() {
		return measureTdxQemuLegacy(bnd, comp, fw, tdvfMeta)
	}
	return measureTdxQemu(bnd, comp, fw, tdvfMeta)
}

func efiBootOption() []byte {
	// EfiBootManagerGetBootManagerMenu(), LOAD_OPTION_ACTIVE | HIDDEN |
	// CATEGORY_APP, with the FV and file GUIDs from the pinned OVMF build.
	data := binary.LittleEndian.AppendUint32(nil, 0x109)
	data = binary.LittleEndian.AppendUint16(data, 44)
	for _, c := range utf16.Encode([]rune("BootManagerMenuApp\x00")) {
		data = binary.LittleEndian.AppendUint16(data, c)
	}
	data = append(data, 4, 7, 20, 0)
	data = append(data, encodeGUID("7cb8bdc9-f8eb-4f34-aaea-3ee4af6516a1")...)
	data = append(data, 4, 6, 20, 0)
	data = append(data, encodeGUID("eec25bdc-67f2-4d95-b1d5-f81b2039d11d")...)
	return append(data, 0x7f, 0xff, 4, 0)
}

// efiLogSeparator is the separator used in EFI event logs.
var efiLogSeparator = measureSha384([]byte{0, 0, 0, 0})

func tdxRtmr0Log(inputs *TdxPlatformInputs, cfv []byte) [][]byte {
	rtmr0Log := make([][]byte, 0, 14+len(inputs.FwCfg))
	rtmr0Log = append(rtmr0Log,
		measureSha384(inputs.TDHOB),
		measureSha384(cfv),
	)

	for _, entry := range inputs.FwCfg {
		rtmr0Log = append(rtmr0Log, measureSha384(entry.Data))
	}

	rtmr0Log = append(rtmr0Log,
		measureEfiVariableValue(efiVariableGlobal, "SecureBoot", []byte{0}),
		measureEfiVariableValue(efiVariableGlobal, "PK", nil),
		measureEfiVariableValue(efiVariableGlobal, "KEK", nil),
		measureEfiVariableValue(efiVariableImageSecurity, "db", nil),
		measureEfiVariableValue(efiVariableImageSecurity, "dbx", nil),
		efiLogSeparator,
		measureSha384(inputs.Loader),
		measureSha384(inputs.RSDP),
		measureSha384(inputs.ACPI),
		measureSha384(inputs.SMBIOS),
		measureSha384([]byte{0, 0}),
		measureSha384(efiBootOption()),
	)

	return rtmr0Log
}

func tdxRtmr1Log(bnd *bundle.Bundle, comp *bundle.Component) ([][]byte, error) {
	kernelAuthenticodeHash, err := measureTdxQemuKernelImage(bnd, comp)
	if err != nil {
		return nil, err
	}

	rtmr1Log := [][]byte{
		kernelAuthenticodeHash,
		measureSha384([]byte("Calling EFI Application from Boot Option")),
		efiLogSeparator,
		measureSha384([]byte("Exit Boot Services Invocation")),
		measureSha384([]byte("Exit Boot Services Returned with Success")),
	}
	return rtmr1Log, nil
}

func tdxRtmr2Log(comp *bundle.Component) [][]byte {
	kernelCmdline := strings.Join(comp.TDX.ExtraKernelOptions, " ")
	rtmr2Log := [][]byte{
		measureTdxKernelCmdline(kernelCmdline),
	}
	return rtmr2Log
}

func measureTdxQemu(bnd *bundle.Bundle, comp *bundle.Component, fw []byte, tdvfMeta *tdvfMetadata) ([]bundle.Identity, error) {
	inputs, err := generateTdxPlatformInputs(bnd, comp, fw, tdvfMeta)
	if err != nil {
		return nil, err
	}

	// Ensure that the CFV is present and represents an empty configuration volume.
	var cfv []byte
	for _, s := range tdvfMeta.Sections {
		if s.SecType != tdvfSectionCFV {
			continue
		}
		if cfv != nil {
			return nil, fmt.Errorf("duplicate TDVF configuration volume")
		}
		cfv = fw[s.DataOffset : uint64(s.DataOffset)+uint64(s.RawDataSize)]
	}
	emptyCFV, _ := hex.DecodeString("344BC51C980BA621AAA00DA3ED7436F7D6E549197DFE699515DFA2C6583D95E6412AF21C097D473155875FFD561D6790")
	if !bytes.Equal(measureSha384(cfv), emptyCFV) {
		return nil, fmt.Errorf("TDX profile requires the pristine configuration volume")
	}

	// RTMR1.
	rtmr1Log, err := tdxRtmr1Log(bnd, comp)
	if err != nil {
		return nil, err
	}
	rtmr1 := measureLog(rtmr1Log)

	// RTMR2.
	rtmr2Log := tdxRtmr2Log(comp)
	rtmr2 := measureLog(rtmr2Log)

	// RTMR3.
	var rtmr3 [48]byte
	// All-zero for now.

	identities := make([]bundle.Identity, 0, len(inputs))
	for _, input := range inputs {
		// RTMR0.
		rtmr0Log := tdxRtmr0Log(input, cfv)
		rtmr0 := measureLog(rtmr0Log)

		variant := tdxProfileSpecs[input.Hypervisor].MRTDVariant
		mrtd := tdvfMeta.computeMrtd(fw, variant)
		eid := pcs.ComputeTdEnclaveIdentity(mrtd, rtmr0, rtmr1, rtmr2, rtmr3[:])

		identities = append(identities, bundle.Identity{
			Hypervisor: input.Hypervisor,
			Enclave:    eid,
		})
	}
	return identities, nil
}

func measureTdxQemuLegacy(bnd *bundle.Bundle, comp *bundle.Component, fw []byte, tdvfMeta *tdvfMetadata) ([]bundle.Identity, error) {
	// RTMR0.
	tdHobHash, err := measureTdxQemuTdHob(&comp.TDX.Resources, tdvfMeta)
	if err != nil {
		return nil, err
	}
	cfvImageHash, _ := hex.DecodeString("344BC51C980BA621AAA00DA3ED7436F7D6E549197DFE699515DFA2C6583D95E6412AF21C097D473155875FFD561D6790")
	boot000Hash, _ := hex.DecodeString("23ADA07F5261F12F34A0BD8E46760962D6B4D576A416F1FEA1C64BC656B1D28EACF7047AE6E967C58FD2A98BFA74C298")
	acpiTablesHash, acpiRsdpHash, acpiLoaderHash, err := measureTdxQemuAcpiTablesLegacy(&comp.TDX.Resources)
	if err != nil {
		return nil, err
	}

	rtmr0Log := append([][]byte{},
		tdHobHash,
		cfvImageHash,
		measureEfiVariableValue(efiVariableGlobal, "SecureBoot", nil),
		measureEfiVariableValue(efiVariableGlobal, "PK", nil),
		measureEfiVariableValue(efiVariableGlobal, "KEK", nil),
		measureEfiVariableValue(efiVariableImageSecurity, "db", nil),
		measureEfiVariableValue(efiVariableImageSecurity, "dbx", nil),
		efiLogSeparator,
		acpiLoaderHash,
		acpiRsdpHash,
		acpiTablesHash,
		measureSha384([]byte{0x00, 0x00}), // BootOrder
		boot000Hash,                       // Boot000
		efiLogSeparator,
	)
	rtmr0 := measureLog(rtmr0Log)

	// RTMR1.
	kernelAuthenticodeHash, err := measureTdxQemuKernelImage(bnd, comp)
	if err != nil {
		return nil, err
	}
	rtmr1Log := append([][]byte{},
		kernelAuthenticodeHash,
		measureSha384([]byte("Calling EFI Application from Boot Option")),
		// Note that there is no separator here.
		measureSha384([]byte("Exit Boot Services Invocation")),
		measureSha384([]byte("Exit Boot Services Returned with Success")),
	)
	rtmr1 := measureLog(rtmr1Log)

	// RTMR2.
	rtmr2Log := tdxRtmr2Log(comp)
	rtmr2 := measureLog(rtmr2Log)

	// RTMR3.
	var rtmr3 [48]byte
	// All-zero for now.

	// Compute MRTD for all known QEMU variants as there are unfortunately different
	// implementations.
	ids := make([]bundle.Identity, 0, 2)
	for _, variant := range []int{
		mrtdVariantTwoPass,
		mrtdVariantSinglePass,
	} {
		mrtd := tdvfMeta.computeMrtd(fw, variant)
		eid := pcs.ComputeTdEnclaveIdentity(mrtd, rtmr0, rtmr1, rtmr2, rtmr3[:])

		ids = append(ids, bundle.Identity{
			Hypervisor: fmt.Sprintf("qemu/v%d", variant),
			Enclave:    eid,
		})
	}
	return ids, nil
}
