package encoding

import (
	"bytes"
	"compress/flate"
	"io"
)

func deflate(compressed []byte) ([]byte, error) {
	zr := flate.NewReader(bytes.NewReader(compressed))
	defer func() {
		err := zr.Close()
		if err != nil {
			panic(err)
		}
	}()

	return io.ReadAll(zr)
}
