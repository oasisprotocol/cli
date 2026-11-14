package measurement

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
)

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

func profileBootOption() []byte {
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

func profileLogs(inputs *TdxPlatformInputs, cfv, kernel []byte, cmdline string) ([][]byte, [][]byte, [][]byte) {
	separator := measureSha384([]byte{0, 0, 0, 0})
	r0 := make([][]byte, 0, 14+len(inputs.FwCfg))
	r0 = append(r0, measureSha384(inputs.TDHOB), measureSha384(cfv))
	for _, entry := range inputs.FwCfg {
		r0 = append(r0, measureSha384(entry.Data))
	}
	const global = "8be4df61-93ca-11d2-aa0d-00e098032b8c"
	const imageSecurity = "d719b2cb-3d3a-4596-a3bc-dad00e67656f"
	r0 = append(r0,
		measureEfiVariableValue(global, "SecureBoot", []byte{0}),
		measureEfiVariableValue(global, "PK", nil),
		measureEfiVariableValue(global, "KEK", nil),
		measureEfiVariableValue(imageSecurity, "db", nil),
		measureEfiVariableValue(imageSecurity, "dbx", nil),
		separator,
		measureSha384(inputs.Loader),
		measureSha384(inputs.RSDP),
		measureSha384(inputs.ACPI),
		measureSha384(inputs.SMBIOS),
		measureSha384([]byte{0, 0}),
		measureSha384(profileBootOption()),
	)
	r1 := [][]byte{
		kernel,
		measureSha384([]byte("Calling EFI Application from Boot Option")),
		separator,
		measureSha384([]byte("Exit Boot Services Invocation")),
		measureSha384([]byte("Exit Boot Services Returned with Success")),
	}
	r2 := [][]byte{
		measureTdxKernelCmdline(cmdline),
	}
	return r0, r1, r2
}

func measureTdxProfile(bnd *bundle.Bundle, comp *bundle.Component, fw []byte, meta *tdvfMetadata) ([]bundle.Identity, error) {
	inputs, err := GenerateTdxPlatformInputs(bnd, comp)
	if err != nil {
		return nil, err
	}
	var cfv []byte
	for _, s := range meta.sections {
		if s.secType != 1 {
			continue
		}
		if cfv != nil {
			return nil, fmt.Errorf("duplicate TDVF configuration volume")
		}
		cfv = fw[s.dataOffset : uint64(s.dataOffset)+uint64(s.rawDataSize)]
	}
	// This profile uses a fresh, empty variable store. Enrolled keys, persistent
	// variables and custom Secure Boot policy require a different profile ABI.
	emptyCFV, _ := hex.DecodeString("344bc51c980ba621aaa00da3ed7436f7d6e549197dfe699515dfa2c6583d95e6412af21c097d473155875ffd561d6790")
	if !bytes.Equal(measureSha384(cfv), emptyCFV) {
		return nil, fmt.Errorf("TDX profile requires the pristine configuration volume")
	}
	kernel, err := measureTdxQemuKernelImage(bnd, comp)
	if err != nil {
		return nil, err
	}
	var r3 [48]byte
	identities := make([]bundle.Identity, 0, len(inputs))
	for i := range inputs {
		input := &inputs[i]
		r0, r1, r2 := profileLogs(input, cfv, kernel, strings.Join(comp.TDX.ExtraKernelOptions, " "))
		variant := tdxProfileSpecs[input.Hypervisor].MRTDVariant
		identities = append(identities, bundle.Identity{
			Hypervisor: input.Hypervisor,
			Enclave:    computeEnclaveIdentity(meta.computeMrtd(fw, variant), measureLog(r0), measureLog(r1), measureLog(r2), r3[:]),
		})
	}
	return identities, nil
}
