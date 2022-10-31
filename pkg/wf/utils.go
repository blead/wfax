package wf

import (
	"archive/zip"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func sha256Checksum(reader io.Reader) ([]byte, error) {
	h := sha256.New()
	_, err := io.Copy(h, reader)
	return h.Sum(nil), err
}

func sha1Digest(s string, salt string) (string, error) {
	h := sha1.New()
	_, err := h.Write([]byte(s + salt))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func unzip(src io.ReaderAt, size int64, dest string, modPath func(string) string) error {
	archive, err := zip.NewReader(src, size)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dest, 0755)
	if err != nil {
		return err
	}

	extractFile := func(zf *zip.File) error {
		zdata, err := zf.Open()
		if err != nil {
			return err
		}
		defer func() {
			err := zdata.Close()
			if err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, zf.Name)

		// check for directory traversal (Zip Slip)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}
		path = modPath(path)

		if zf.FileInfo().IsDir() {
			return os.MkdirAll(path, zf.Mode())
		}

		err = os.MkdirAll(filepath.Dir(path), zf.Mode())
		if err != nil {
			return err
		}

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, zf.Mode())
		if err != nil {
			return err
		}
		defer func() {
			err := f.Close()
			if err != nil {
				panic(err)
			}
		}()

		_, err = io.Copy(f, zdata)
		return err
	}

	for _, zf := range archive.File {
		err := extractFile(zf)
		if err != nil {
			return err
		}
	}

	return nil
}
