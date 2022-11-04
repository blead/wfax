package encoding

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"strings"

	omap "github.com/iancoleman/orderedmap"
)

type orderedMapOffset struct {
	KeyEndOffset   int32
	ValueEndOffset int32
}

func readOrderedMap(raw []byte, flattenCSV bool) (json.Marshaler, error) {
	// data can be plain zlib-compressed csv or map structure
	data, err := readZlib(raw)
	if err == nil {
		// data is zlib-compressed csv
		r := csv.NewReader(bytes.NewReader(data))
		rec, err := r.ReadAll()
		if err != nil {
			return nil, err
		}

		output, err := jsonMarshalNoEscape(
			func() any {
				if flattenCSV {
					return Flatten(rec)
				}
				return rec
			}(),
		)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(output), nil
	}

	// map structure
	// first 4 bytes = header size in little endian
	var headerSize int32
	err = binary.Read(bytes.NewReader(raw[0:4]), binary.LittleEndian, &headerSize)
	if err != nil {
		return nil, err
	}

	// header is zlib-compressed
	header, err := readZlib(raw[4 : 4+headerSize])
	if err != nil {
		return nil, err
	}

	// first 4 bytes of header = number of mapped entries
	var entriesCount int32
	err = binary.Read(bytes.NewReader(header[0:4]), binary.LittleEndian, &entriesCount)
	if err != nil {
		return nil, err
	}

	// offsets = header[4:4 + entriesCount*8], each entry = (keyEndOffset, valueEndOffset) (4 + 4 bytes)
	var offsets []*orderedMapOffset
	for i := int32(0); i < entriesCount; i++ {
		offset := orderedMapOffset{}
		err = binary.Read(bytes.NewReader(header[4+i*8:4+i*8+8]), binary.LittleEndian, &offset)
		if err != nil {
			return nil, err
		}
		offsets = append(offsets, &offset)
	}

	// keySection = header[4 + entriesCount*8], each key = keySection[prevKeyEndOffset:KeyEndOffset]
	// valueSection = header[4 + headerSize], each value = valueSection[prevValueEndOffset:ValueEndOffset]
	keySection := header[4+entriesCount*8:]
	valueSection := raw[4+headerSize:]
	currentKeyOffset := int32(0)
	currentValueOffset := int32(0)
	output := omap.New()
	output.SetEscapeHTML(false)
	for _, offset := range offsets {
		key := keySection[currentKeyOffset:offset.KeyEndOffset]
		value := valueSection[currentValueOffset:offset.ValueEndOffset]
		currentKeyOffset = offset.KeyEndOffset
		currentValueOffset = offset.ValueEndOffset

		// value is a nested orderedmap
		om, err := readOrderedMap(value, flattenCSV)
		if err != nil {
			return nil, err
		}
		output.Set(string(key), om)
	}

	return output, nil
}

// OrderedmapToJSON converts WF orderedmap to JSON
func OrderedmapToJSON(raw []byte, indent int, flattenCSV bool) ([]byte, error) {
	om, err := readOrderedMap(raw, flattenCSV)
	if err != nil {
		return nil, err
	}

	js, err := jsonMarshalNoEscape(om)
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	err = json.Indent(&output, js, "", strings.Repeat(" ", indent))

	return output.Bytes(), err
}
