package ledger

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

func getAdr0008Path(number uint32) []uint32 {
	return []uint32{44, 474, number}
}

func getLegacyPath(number uint32) []uint32 {
	return []uint32{44, 474, 0, 0, number}
}

func getBip44Path(number uint32) []uint32 {
	return []uint32{44, 60, 0, 0, number}
}

func getSerializedPath(path []uint32) ([]byte, error) {
	message := make([]byte, 4*len(path))
	switch len(path) {
	case 5:
		// Legacy derivation path.
	case 3:
		// ADR-0008 derivation path.
	default:
		return nil, fmt.Errorf("path should contain either 5 or 3 elements")
	}

	for index, element := range path {
		pos := index * 4
		value := element | 0x80000000 // Harden all components.
		binary.LittleEndian.PutUint32(message[pos:], value)
	}
	return message, nil
}

func getSerializedBip44Path(path []uint32) ([]byte, error) {
	message := make([]byte, 4*len(path))
	switch len(path) {
	case 5:
		// BIP-44 derivation path has always 5 elements.
	default:
		return nil, fmt.Errorf("path should contain 5 elements")
	}

	// First three elements are hardened
	for index, element := range path[:3] {
		pos := index * 4
		value := element | 0x80000000 // Harden all components.
		binary.LittleEndian.PutUint32(message[pos:], value)
	}
	return message, nil
}

func prepareChunks(pathBytes, context, message []byte, chunkSize int, ctxLen bool) ([][]byte, error) {
	var body []byte
	if ctxLen {
		if len(context) > 255 {
			return nil, fmt.Errorf("maximum supported context size is 255 bytes")
		}

		body = []byte{byte(len(context))}
	}
	body = append(body, context...)
	body = append(body, message...)

	packetCount := 1 + len(body)/chunkSize
	if len(body)%chunkSize > 0 {
		packetCount++
	}

	chunks := make([][]byte, 0, packetCount)
	chunks = append(chunks, pathBytes) // First chunk is path.

	r := bytes.NewReader(body)
readLoop:
	for {
		toAppend := make([]byte, chunkSize)
		n, err := r.Read(toAppend)
		if n > 0 {
			// Note: n == 0 only when EOF.
			chunks = append(chunks, toAppend[:n])
		}
		switch err {
		case nil:
		case io.EOF:
			break readLoop
		default:
			// This can never happen, but handle it.
			return nil, err
		}
	}

	return chunks, nil
}

func prepareConsensusChunks(pathBytes, context, message []byte, chunkSize int) ([][]byte, error) {
	return prepareChunks(pathBytes, context, message, chunkSize, true)
}

func prepareRuntimeChunks(pathBytes, metadata, message []byte, chunkSize int) ([][]byte, error) {
	return prepareChunks(pathBytes, metadata, message, chunkSize, false)
}
