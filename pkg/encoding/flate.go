package encoding

import (
	"bytes"
	"compress/flate"
	"io"
)

func deflate(compressed []byte) ([]byte, error) {
	zr := flate.NewReader(bytes.NewReader(compressed))
	defer zr.Close()

	return io.ReadAll(zr)
}
