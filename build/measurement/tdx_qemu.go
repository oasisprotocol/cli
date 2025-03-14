package measurement

import (
	"bytes"
	"crypto"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/foxboron/go-uefi/authenticode"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/tuplehash"
	"github.com/oasisprotocol/oasis-core/go/common/sgx"
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

// measureTdxQemuTdHob measures the TD HOB.
func measureTdxQemuTdHob(resources *bundle.TDXResources, meta *tdvfMetadata) []byte {
	// Construct a TD hob in the same way as QEMU does. Note that all fields are little-endian.
	// See: https://github.com/intel-staging/qemu-tdx/blob/tdx-qemu-next/hw/i386/tdvf-hob.c
	var tdHob []byte
	// Discover the TD HOB base address from TDVF metadata.
	tdHobBaseAddr := uint64(0x809000) // TD HOB base address.
	if meta != nil {
		for _, s := range meta.sections {
			if s.secType == tdvfSectionTdHob {
				tdHobBaseAddr = s.memoryAddress
				break
			}
		}
	}

	// Start with EFI_HOB_TYPE_HANDOFF.
	tdHob = append(tdHob,
		0x01, 0x00, // Header.HobType (EFI_HOB_TYPE_HANDOFF)
		0x38, 0x00, // Header.HobLength (56 bytes)
		0x00, 0x00, 0x00, 0x00, // Header.Reserved
		0x09, 0x00, 0x00, 0x00, // Version (EFI_HOB_HANDOFF_TABLE_VERSION)
		0x00, 0x00, 0x00, 0x00, // BootMode
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // EfiMemoryTop
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // EfiMemoryBottom
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // EfiFreeMemoryTop
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // EfiFreeMemoryBottom
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // EfiEndOfHobList (filled later)
	)

	// The rest of the HOBs are EFI_HOB_TYPE_RESOURCE_DESCRIPTOR.
	remainingMemory := resources.Memory * 1024 * 1024 // Convert to bytes.
	addMemoryResourceHob := func(resourceType uint8, start, length uint64) {
		tdHob = append(tdHob,
			0x03, 0x00, // Header.HobType (EFI_HOB_TYPE_RESOURCE_DESCRIPTOR)
			0x30, 0x00, // Header.HobLength (48 bytes)
			0x00, 0x00, 0x00, 0x00, // Header.Reserved
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Owner
			resourceType, 0x00, 0x00, 0x00, // ResourceType
			0x07, 0x00, 0x00, 0x00, // ResourceAttribute
		)

		var val [8]byte
		binary.LittleEndian.PutUint64(val[:], start)
		tdHob = append(tdHob, val[:]...) // PhysicalStart
		binary.LittleEndian.PutUint64(val[:], length)
		tdHob = append(tdHob, val[:]...) // Length

		// Subtract from remaining memory.
		remainingMemory -= length
	}

	addMemoryResourceHob(0x07, 0x0000000000000000, 0x0000000000800000)
	addMemoryResourceHob(0x00, 0x0000000000800000, 0x0000000000006000)
	addMemoryResourceHob(0x07, 0x0000000000806000, 0x0000000000003000)
	addMemoryResourceHob(0x00, 0x0000000000809000, 0x0000000000002000)
	addMemoryResourceHob(0x00, 0x000000000080B000, 0x0000000000002000)
	addMemoryResourceHob(0x07, 0x000000000080D000, 0x0000000000003000)
	addMemoryResourceHob(0x00, 0x0000000000810000, 0x0000000000010000)

	// Handle memory split at 2816 MiB (0xB0000000).
	if resources.Memory >= 2816 {
		addMemoryResourceHob(0x07, 0x0000000000820000, 0x000000007F7E0000)
		addMemoryResourceHob(0x07, 0x0000000100000000, remainingMemory)
	} else {
		addMemoryResourceHob(0x07, 0x0000000000820000, remainingMemory)
	}

	// Update EfiEndOfHobList.
	var val [8]byte
	binary.LittleEndian.PutUint64(val[:], tdHobBaseAddr+uint64(len(tdHob))+8)
	copy(tdHob[48:56], val[:])

	// Measure the TD HOB.
	return measureSha384(tdHob)
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

// measureTdxQemuAcpiTables measures QEMU-generated ACPI tables for TDX.
func measureTdxQemuAcpiTables(resources *bundle.TDXResources) ([]byte, []byte, []byte, error) {
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

// measureTdxEfiVariable measures an EFI variable event.
func measureTdxEfiVariable(vendorGUID string, varName string) []byte {
	var data []byte
	data = append(data, encodeGUID(vendorGUID)...)

	var encLen [8]byte
	binary.LittleEndian.PutUint64(encLen[:], uint64(len(varName)))
	data = append(data, encLen[:]...)
	binary.LittleEndian.PutUint64(encLen[:], 0)
	data = append(data, encLen[:]...)

	// Convert varName to UTF-16LE.
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	xr := transform.NewReader(bytes.NewReader([]byte(varName)), utf16le)
	converted, _ := io.ReadAll(xr)
	data = append(data, converted...)

	return measureSha384(data)
}

const (
	attributeMrExtend   = 0b00000000_00000000_00000000_00000001
	attributePageAug    = 0b00000000_00000000_00000000_00000010
	pageSize            = 0x1000
	mrExtendGranularity = 0x100

	tdvfSectionTdHob = 0x02
)

type tdvfSection struct {
	dataOffset     uint32
	rawDataSize    uint32
	memoryAddress  uint64
	memoryDataSize uint64
	secType        uint32
	attributes     uint32
}

type tdvfMetadata struct {
	sections []*tdvfSection
}

const (
	mrtdVariantTwoPass    = 0
	mrtdVariantSinglePass = 1
)

func (m *tdvfMetadata) computeMrtd(fw []byte, variant int) []byte {
	h := sha512.New384()

	memPageAdd := func(s *tdvfSection, page uint64) {
		if s.attributes&attributePageAug == 0 {
			// Use TDCALL [TDH.MEM.PAGE.ADD].
			//
			// Byte 0 through 11 contain the ASCII string 'MEM.PAGE.ADD'.
			// Byte 16 through 23 contain the GPA (in little-endian format).
			// All the other bytes contain 0.
			var buf [128]byte
			copy(buf[:12], []byte("MEM.PAGE.ADD"))
			binary.LittleEndian.PutUint64(buf[16:24], s.memoryAddress+page*pageSize)
			_, _ = h.Write(buf[:])
		}
	}

	mrExtend := func(s *tdvfSection, page uint64) {
		if s.attributes&attributeMrExtend != 0 {
			// Need TDCALL [TDH.MR.EXTEND].
			for i := range pageSize / mrExtendGranularity {
				// Byte 0 through 8 contain the ASCII string 'MR.EXTEND'.
				// Byte 16 through 23 contain the GPA (in little-endian format).
				// All the other bytes contain 0.
				var buf [128]byte
				copy(buf[:9], []byte("MR.EXTEND"))
				binary.LittleEndian.PutUint64(buf[16:24], s.memoryAddress+page*pageSize+uint64(i*mrExtendGranularity)) //nolint: gosec
				_, _ = h.Write(buf[:])

				// The other two extension buffers contain the chunk’s content.
				chunkOffset := int(s.dataOffset) + int(page*pageSize) + i*mrExtendGranularity //nolint: gosec
				_, _ = h.Write(fw[chunkOffset : chunkOffset+mrExtendGranularity])
			}
		}
	}

	for _, s := range m.sections {
		numPages := s.memoryDataSize / pageSize

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

// parseTdvfMetadata parses the TDVF metadata from the firmware blob.
//
// See Section 11 of "Intel TDX Virtual Firmware Design Guide" for details.
func parseTdvfMetadata(fw []byte) (*tdvfMetadata, error) {
	const (
		tdxMetadataOffsetGUID = "e47a6535-984a-4798-865e-4685a7bf8ec2"
		tdxMetadataVersion    = 1
		tdvfSignature         = "TDVF"
		tableFooterGUID       = "96b582de-1fb2-45f7-baea-a366c55a082d"
		bytesAfterTableFooter = 32
	)

	offset := len(fw) - bytesAfterTableFooter
	encodedFooterGUID := encodeGUID(tableFooterGUID)
	guid := fw[offset-16 : offset]
	tablesLen := int(binary.LittleEndian.Uint16(fw[offset-16-2 : offset-16]))
	if !bytes.Equal(guid, encodedFooterGUID) {
		return nil, fmt.Errorf("malformed OVMF table footer")
	}
	if tablesLen == 0 || tablesLen > offset-16-2 {
		return nil, fmt.Errorf("malformed OVMF table footer")
	}
	tables := fw[offset-16-2-tablesLen : offset-16-2]
	offset = len(tables)

	// Find TDVF metadata table in OVMF, starting at the end.
	var data []byte
	encodedGUID := encodeGUID(tdxMetadataOffsetGUID)
	for {
		if offset < 18 {
			return nil, fmt.Errorf("missing TDVF metadata in firmware")
		}

		// The data structure is:
		//
		//   arbitrary length data
		//   2 byte length of entire entry
		//   16 byte GUID
		//
		guid = tables[offset-16 : offset]
		entryLen := int(binary.LittleEndian.Uint16(tables[offset-16-2 : offset-16]))
		if offset < 18+entryLen {
			return nil, fmt.Errorf("malformed OVMF table in firmware at offset %d", offset)
		}

		if bytes.Equal(guid, encodedGUID) {
			data = tables[offset-18-entryLen : offset-18]
			break
		}

		offset -= entryLen
	}
	if data == nil {
		return nil, fmt.Errorf("missing TDVF metadata in firmware")
	}

	// Extract and parse TDVF metadata descriptor:
	//
	//   4 byte signature
	//   4 byte length
	//   4 byte version
	//   4 byte number of section entries
	//   32 byte each section * number of sections
	//
	tdvfMetaOffset := int(binary.LittleEndian.Uint32(data[len(data)-4:]))
	tdvfMetaOffset = len(fw) - tdvfMetaOffset
	tdvfMetaDesc := fw[tdvfMetaOffset : tdvfMetaOffset+16]
	if string(tdvfMetaDesc[:4]) != tdvfSignature {
		return nil, fmt.Errorf("malformed TDVF metadata descriptor in firmware")
	}
	tdvfVersion := binary.LittleEndian.Uint32(tdvfMetaDesc[8:12])
	tdvfNumberOfSectionEntries := int(binary.LittleEndian.Uint32(tdvfMetaDesc[12:16]))
	if tdvfVersion != 1 {
		return nil, fmt.Errorf("unsupported TDVF metadata descriptor version in firmware")
	}

	// Parse section entries.
	var meta tdvfMetadata
	for section := range tdvfNumberOfSectionEntries {
		secOffset := tdvfMetaOffset + 16 + 32*section
		secData := fw[secOffset : secOffset+32]

		s := &tdvfSection{
			dataOffset:     binary.LittleEndian.Uint32(secData[:4]),
			rawDataSize:    binary.LittleEndian.Uint32(secData[4:8]),
			memoryAddress:  binary.LittleEndian.Uint64(secData[8:16]),
			memoryDataSize: binary.LittleEndian.Uint64(secData[16:24]),
			secType:        binary.LittleEndian.Uint32(secData[24:28]),
			attributes:     binary.LittleEndian.Uint32(secData[28:32]),
		}

		// Sanity check section.
		if s.memoryAddress%pageSize != 0 {
			return nil, fmt.Errorf("TDVF metadata section %d has non-aligned memory address", section)
		}
		if s.memoryDataSize < uint64(s.rawDataSize) {
			return nil, fmt.Errorf("TDVF metadata section %d memory data size is less than raw data size", section)
		}
		if s.memoryDataSize%pageSize != 0 {
			return nil, fmt.Errorf("TDVF metadata section %d has non-aligned memory data size", section)
		}
		if s.attributes&attributeMrExtend != 0 && uint64(s.rawDataSize) < s.memoryDataSize {
			return nil, fmt.Errorf("TDVF metadata section %d raw data size is less than memory data size", section)
		}

		meta.sections = append(meta.sections, s)
	}
	return &meta, nil
}

// TODO: Expose this in Oasis Core instead of reimplementing it here.
func computeEnclaveIdentity(
	mrtd []byte,
	rtmr0 []byte,
	rtmr1 []byte,
	rtmr2 []byte,
	rtmr3 []byte,
) sgx.EnclaveIdentity {
	var (
		zeroMrSigner sgx.MrSigner
		mrEnclave    sgx.MrEnclave
	)
	h := tuplehash.New256(32, []byte(pcs.TdEnclaveIdentityContext))
	_, _ = h.Write(mrtd)
	_, _ = h.Write(rtmr0)
	_, _ = h.Write(rtmr1)
	_, _ = h.Write(rtmr2)
	_, _ = h.Write(rtmr3)
	rawMrEnclave := h.Sum(nil)
	copy(mrEnclave[:], rawMrEnclave)

	return sgx.EnclaveIdentity{
		MrEnclave: mrEnclave,
		MrSigner:  zeroMrSigner, // All-zero MRSIGNER (invalid in SGX).
	}
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

	// RTMR0.
	tdHobHash := measureTdxQemuTdHob(&comp.TDX.Resources, tdvfMeta)
	cfvImageHash, _ := hex.DecodeString("344BC51C980BA621AAA00DA3ED7436F7D6E549197DFE699515DFA2C6583D95E6412AF21C097D473155875FFD561D6790")
	boot000Hash, _ := hex.DecodeString("23ADA07F5261F12F34A0BD8E46760962D6B4D576A416F1FEA1C64BC656B1D28EACF7047AE6E967C58FD2A98BFA74C298")
	acpiTablesHash, acpiRsdpHash, acpiLoaderHash, err := measureTdxQemuAcpiTables(&comp.TDX.Resources)
	if err != nil {
		return nil, err
	}

	rtmr0Log := append([][]byte{},
		tdHobHash,
		cfvImageHash,
		measureTdxEfiVariable("8BE4DF61-93CA-11D2-AA0D-00E098032B8C", "SecureBoot"),
		measureTdxEfiVariable("8BE4DF61-93CA-11D2-AA0D-00E098032B8C", "PK"),
		measureTdxEfiVariable("8BE4DF61-93CA-11D2-AA0D-00E098032B8C", "KEK"),
		measureTdxEfiVariable("D719B2CB-3D3A-4596-A3BC-DAD00E67656F", "db"),
		measureTdxEfiVariable("D719B2CB-3D3A-4596-A3BC-DAD00E67656F", "dbx"),
		measureSha384([]byte{0x00, 0x00, 0x00, 0x00}), // Separator.
		acpiLoaderHash,
		acpiRsdpHash,
		acpiTablesHash,
		measureSha384([]byte{0x00, 0x00}), // BootOrder
		boot000Hash,                       // Boot000
		measureSha384([]byte{0x00, 0x00, 0x00, 0x00}), // Separator.
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
		measureSha384([]byte("Exit Boot Services Invocation")),
		measureSha384([]byte("Exit Boot Services Returned with Success")),
	)
	rtmr1 := measureLog(rtmr1Log)

	// RTMR2.
	kernelCmdline := strings.Join(comp.TDX.ExtraKernelOptions, " ")
	rtmr2log := append([][]byte{},
		measureTdxKernelCmdline(kernelCmdline),
	)
	rtmr2 := measureLog(rtmr2log)

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
		eid := computeEnclaveIdentity(mrtd, rtmr0, rtmr1, rtmr2, rtmr3[:])

		ids = append(ids, bundle.Identity{
			Hypervisor: fmt.Sprintf("qemu/v%d", variant),
			Enclave:    eid,
		})
	}
	return ids, nil
}
