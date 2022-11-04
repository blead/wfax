package encoding

import (
	"bytes"
	"compress/zlib"
	"io"
)

func readZlib(compressed []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}
