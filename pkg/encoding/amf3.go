package encoding

import (
	"bytes"
	"encoding/json"
	"strings"

	amf "github.com/remyoudompheng/goamf"
)

// Amf3ToJSON converts Adobe AMF3 to JSON
func Amf3ToJSON(raw []byte, indent int) ([]byte, error) {
	data, err := deflate(raw)
	if err != nil {
		return nil, err
	}

	d := amf.NewDecoder()
	amf3, err := d.DecodeAmf3(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	js, err := jsonMarshalNoEscape(amf3)
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	err = json.Indent(&output, js, "", strings.Repeat(" ", indent))

	return output.Bytes(), err
}
