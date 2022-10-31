package wf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/blead/wfax/assets"
	"github.com/blead/wfax/pkg/concurrency"
)

const (
	dumpDir             = "dump"
	dumpAssetDir        = "upload"
	outputDir           = "output"
	outputOrderedMapDir = "orderedmap"
	outputAssetsDir     = "assets"
)

func getInitialFilePaths() ([][]byte, error) {
	var paths [][]byte
	pattern := regexp.MustCompile(`"path":"(.*)"`)
	matches := pattern.FindAllSubmatch(assets.BootFFC6, -1)
	for _, match := range matches {
		paths = append(paths, match[1])
	}
	return paths, nil
}

func addExt(p string, ext string) string {
	return path.Clean(p + ext)
}

func toMasterTablePath(p string) string {
	return path.Clean(addExt(path.Join("master", p), ".orderedmap"))
}

func findAllPaths(b []byte) ([][]byte, error) {
	pattern := regexp.MustCompile(`[.$a-zA-Z_0-9]+?/[.$a-zA-Z_0-9/]+`)
	return pattern.FindAll(b, -1), nil
}

type extractParams struct {
	src     string
	dest    string
	extract func([]byte) ([]byte, error)
	output  func([]byte) ([][]byte, error)
}

func extractFile(i *concurrency.Item) (interface{}, error) {
	params, ok := i.Data.(extractParams)
	if !ok {
		return nil, fmt.Errorf("unable to cast item.Data to extractParams: %v", *i)
	}

	srcFile, err := os.Open(params.src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() {
		err := srcFile.Close()
		if err != nil {
			panic(err)
		}
	}()

	data, err := io.ReadAll(srcFile)
	if err != nil {
		return nil, err
	}
	data, err = params.extract(data)
	if err != nil {
		return nil, err
	}

	os.MkdirAll(filepath.Dir(params.dest), 0755)
	destFile, err := os.OpenFile(params.dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := destFile.Close()
		if err != nil {
			panic(err)
		}
	}()

	_, err = io.Copy(destFile, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	if params.output != nil {
		return params.output(data)
	}
	return nil, nil
}
