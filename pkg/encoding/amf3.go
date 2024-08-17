package encoding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	amf "github.com/remyoudompheng/goamf"
)

// Amf3ToJSON converts Adobe AMF3 to JSON.
func Amf3ToJSON(raw []byte, indent int) ([]byte, error) {
	data, err := inflate(raw)
	if err != nil {
		return nil, fmt.Errorf("flate decompress error, %w", err)
	}

	d := amf.NewDecoder()
	amf3, err := d.DecodeAmf3(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("amf3 decode error, %w", err)
	}

	js, err := jsonMarshalNoEscape(initAmfTypes(amf3))
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	err = json.Indent(&output, js, "", strings.Repeat(" ", indent))

	return output.Bytes(), err
}

// JSONToAmf3 converts JSON to Adobe AMF3
func JSONToAmf3(js []byte) ([]byte, error) {
	var data any
	err := json.Unmarshal(js, &data)
	if err != nil {
		return nil, err
	}

	var amf3 bytes.Buffer
	e := new(amf.Encoder)
	_, err = e.EncodeAmf3(&amf3, initAmfTypes(data))
	if err != nil {
		return nil, err
	}

	return deflate(amf3.Bytes())
}

// 1. empty arrays/maps returned from goamf.Decoder#DecodeAmf3 are nil pointers so they need to be initialized before marshalling to JSON
// 2. primitive map[string]any needs to be converted to goamf.Object first for goamf.Encoder#EncodeAmf3 to work properly
func initAmfTypes(p any) any {
	switch v := p.(type) {
	case map[string]any, amf.Object:
		val, ok := v.(map[string]any)
		if !ok {
			val = map[string]any(v.(amf.Object))
		}

		obj := make(amf.Object)
		for key, value := range val {
			obj[key] = initAmfTypes(value)
		}
		return obj
	case []any, amf.Array:
		val, ok := v.([]any)
		if !ok {
			val = []any(v.(amf.Array))
		}

		arr := make(amf.Array, len(val))
		for i, value := range val {
			arr[i] = initAmfTypes(value)
		}
		return arr
	default:
		return v
	}
}
