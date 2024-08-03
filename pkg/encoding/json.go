package encoding

import (
	"bytes"
	"encoding/json"
	"fmt"

	omap "github.com/iancoleman/orderedmap"
)

func jsonMarshalNoEscape(v any) ([]byte, error) {
	output := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(v)
	return output.Bytes(), err
}

func shallowUnmarshalJSONObject(b []byte) (*omap.OrderedMap, error) {
	dec := json.NewDecoder(bytes.NewReader(b))

	// read '{'
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := t.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("shallowUnmarshalJSONObject: unexpected token, expected='{', found='%s', obj=%s", delim.String(), string(b))
	}

	o := omap.New()

	for dec.More() {
		// read key
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key := t.(string)

		// read value
		var value json.RawMessage
		err = dec.Decode(&value)
		if err != nil {
			return nil, err
		}

		o.Set(key, value)
	}

	// read '}'
	t, err = dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok = t.(json.Delim)
	if !ok || delim != '}' {
		return nil, fmt.Errorf("shallowUnmarshalJSONObject: unexpected token, expected='}', found='%s', obj=%s", delim.String(), string(b))
	}

	return o, nil
}
