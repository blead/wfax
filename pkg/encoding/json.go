package encoding

import (
	"bytes"
	"encoding/json"
)

func jsonMarshalNoEscape(v any) ([]byte, error) {
	output := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(v)
	return output.Bytes(), err
}
