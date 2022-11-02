package encoding

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"

	omap "github.com/iancoleman/orderedmap"
)

func flatten(slices [][]string) []string {
	total := 0
	for _, slice := range slices {
		total += len(slice)
	}
	flat := make([]string, total)
	i := 0
	for _, slice := range slices {
		i += copy(flat[i:], slice)
	}
	return flat
}

type orderedMapOffset struct {
	KeyEndOffset   int32
	ValueEndOffset int32
}

func readOrderedMap(raw []byte) (json.Marshaler, error) {
	data, err := readZlib(raw)
	if err == nil {
		// data is zlib-compressed csv
		r := csv.NewReader(bytes.NewReader(data))
		rec, err := r.ReadAll()
		if err != nil {
			return nil, err
		}
		output, err := jsonMarshalNoEscape(flatten(rec))
		if err != nil {
			return nil, err
		}
		return json.RawMessage(output), nil
	}

	var headerSize int32
	err = binary.Read(bytes.NewReader(raw[0:4]), binary.LittleEndian, &headerSize)
	if err != nil {
		return nil, err
	}

	header, err := readZlib(raw[4 : 4+headerSize])
	if err != nil {
		return nil, err
	}

	var entriesCount int32
	err = binary.Read(bytes.NewReader(header[0:4]), binary.LittleEndian, &entriesCount)
	if err != nil {
		return nil, err
	}

	var offsets []*orderedMapOffset
	for i := int32(0); i < entriesCount; i++ {
		offset := orderedMapOffset{}
		// start at header[4], each entry has 2 values, 4 bytes each
		err = binary.Read(bytes.NewReader(header[4+i*8:4+i*8+8]), binary.LittleEndian, &offset)
		if err != nil {
			return nil, err
		}
		offsets = append(offsets, &offset)
	}

	// start at header[4 + entriesCount*8], each key end at header[4 + entriesCount*8 + Keyendoffset]
	keySection := header[4+entriesCount*8:]
	var keys [][]byte
	currentOffset := int32(0)
	for _, offset := range offsets {
		keys = append(keys, keySection[currentOffset:offset.KeyEndOffset])
		currentOffset = offset.KeyEndOffset
	}

	valueSection := raw[4+headerSize:]
	var values [][]byte
	currentOffset = int32(0)
	for _, offset := range offsets {
		values = append(values, valueSection[currentOffset:offset.ValueEndOffset])
		currentOffset = offset.ValueEndOffset
	}

	output := omap.New()
	output.SetEscapeHTML(false)
	for i, key := range keys {
		value, err := readOrderedMap(values[i])
		if err != nil {
			return nil, err
		}
		output.Set(string(key), value)
	}

	return output, nil
}

// OrderedmapToJSON converts WF orderedmap to JSON
func OrderedmapToJSON(raw []byte) ([]byte, error) {
	om, err := readOrderedMap(raw)
	if err != nil {
		return nil, err
	}
	return jsonMarshalNoEscape(om)
}
