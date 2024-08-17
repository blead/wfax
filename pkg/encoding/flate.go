package encoding

import (
	"bytes"
	"compress/flate"
	"io"
)

// deflate compresses raw bytes
func deflate(raw []byte) ([]byte, error) {
	var output bytes.Buffer
	zw, err := flate.NewWriter(&output, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	defer zw.Close()

	_, err = zw.Write(raw)
	if err != nil {
		return nil, err
	}

	err = zw.Close()
	if err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

// inflate decompresses deflate-encoded bytes
func inflate(compressed []byte) ([]byte, error) {
	zr := flate.NewReader(bytes.NewReader(compressed))
	defer zr.Close()

	return io.ReadAll(zr)
}
