package encoding

import (
	"bytes"

	amf "github.com/remyoudompheng/goamf"
)

// Amf3ToJSON converts Adobe AMF3 to JSON
func Amf3ToJSON(raw []byte) ([]byte, error) {
	data, err := deflate(raw)
	if err != nil {
		return nil, err
	}

	d := amf.NewDecoder()
	output, err := d.DecodeAmf3(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return jsonMarshalNoEscape(output)
}
