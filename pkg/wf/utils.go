package wf

import (
	"archive/zip"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

func addExt(p string, ext string) string {
	return path.Clean(p + ext)
}

func toMasterTablePath(p string) string {
	return path.Clean(addExt(path.Join("master", p), ".orderedmap"))
}

func findAllPaths(b []byte) ([][]byte, error) {
	pattern := regexp.MustCompile(`[.$a-zA-Z_0-9-]+?/[.$a-zA-Z_0-9/-]+`)
	return pattern.FindAll(b, -1), nil
}

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

func lszip(src io.ReaderAt, size int64, modPath func(string) string) ([]string, error) {
	archive, err := zip.NewReader(src, size)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, zf := range archive.File {
		if !zf.FileInfo().IsDir() {
			path := modPath(filepath.Clean(zf.Name))
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func unzip(src io.ReaderAt, size int64, dest string, modPath func(string) string, checkPath func(string) bool) error {
	archive, err := zip.NewReader(src, size)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dest, 0777)
	if err != nil {
		return err
	}

	extractFile := func(zf *zip.File) error {
		zdata, err := zf.Open()
		if err != nil {
			return err
		}
		defer zdata.Close()

		path := modPath(filepath.Clean(zf.Name))
		if !checkPath(path) {
			// skip this file
			return nil
		}

		path = filepath.Join(dest, path)

		// check for directory traversal (Zip Slip)
		if !strings.HasPrefix(path, dest+string(os.PathSeparator)) {
			return fmt.Errorf("unzip: illegal file path: %s", path)
		}

		if zf.FileInfo().IsDir() {
			return os.MkdirAll(path, 0777)
		}

		err = os.MkdirAll(filepath.Dir(path), 0777)
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
				log.Fatalln(err)
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
