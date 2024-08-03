package encoding

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
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

func writeOrderedmap(jsonData []byte) ([]byte, error) {
	// trim potential whitespaces
	data := bytes.TrimLeft(jsonData, "\t\r\n")
	if len(data) == 0 {
		return []byte{}, nil
	}

	// array: convert to zlib-compressed csv
	if data[0] == '[' {
		var rec [][]string
		err := json.Unmarshal(data, &rec)
		if err != nil {
			return nil, err
		}

		var csvData bytes.Buffer
		w := csv.NewWriter(&csvData)
		err = w.WriteAll(rec)
		if err != nil {
			return nil, err
		}

		return writeZlib(csvData.Bytes())
	}

	// object: convert to map structure
	if data[0] == '{' {
		om, err := shallowUnmarshalJSONObject(data)
		if err != nil {
			return nil, err
		}

		var keySection, valueSection []byte
		var offsets []orderedMapOffset
		currentKeyOffset := int32(0)
		currentValueOffset := int32(0)
		keys := om.Keys()

		for _, k := range keys {
			keyBytes := []byte(k)

			v, _ := om.Get(k)
			valueJSON, ok := v.(json.RawMessage)
			if !ok {
				return nil, fmt.Errorf("writeOrderedmap: value type assertion failed, key=%s, data=%s", k, string(data))
			}
			valueBytes, err := writeOrderedmap(valueJSON)
			if err != nil {
				return nil, err
			}

			keySection = append(keySection, keyBytes...)
			valueSection = append(valueSection, valueBytes...)

			offsets = append(offsets, orderedMapOffset{
				KeyEndOffset:   currentKeyOffset + int32(len(keyBytes)),
				ValueEndOffset: currentValueOffset + int32(len(valueBytes)),
			})

			currentKeyOffset += int32(len(keyBytes))
			currentValueOffset += int32(len(valueBytes))
		}

		// header: entriesCount(4) + offsets(4+4)*entriesCount + keySection
		var header bytes.Buffer
		binary.Write(&header, binary.LittleEndian, int32(len(offsets)))
		for _, offset := range offsets {
			binary.Write(&header, binary.LittleEndian, offset)
		}
		header.Write(keySection)

		compressedHeader, err := writeZlib(header.Bytes())
		if err != nil {
			return nil, err
		}

		// orderedmap: headerSize(4) + compressedHeader(headerSize) + valueSection
		var output bytes.Buffer
		binary.Write(&output, binary.LittleEndian, int32(len(compressedHeader)))
		output.Write(compressedHeader)
		output.Write(valueSection)

		return output.Bytes(), nil
	}

	return nil, fmt.Errorf("writeOrderedmap: unexpected data structure, data=%s", string(data))
}

// OrderedmapToJSON converts WF orderedmap to JSON.
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

// JSONToOrderedmap converts JSON back to WF orderedmap.
func JSONToOrderedmap(jsonData []byte) ([]byte, error) {
	return writeOrderedmap(jsonData)
}
