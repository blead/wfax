package wf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/blead/wfax/pkg/concurrency"
	"github.com/blead/wfax/pkg/encoding"
)

const (
	digestSalt = "K6R9T9Hz22OpeIGEWB0ui6c6PYFQnJGy"
)

// ExtractorConfig is the configuration for the extractor.
type ExtractorConfig struct {
	Workdir     string
	Concurrency int
}

// DefaultExtractorConfig generates a default configuration.
func DefaultExtractorConfig() *ExtractorConfig {
	return &ExtractorConfig{
		Workdir:     "",
		Concurrency: 5,
	}
}

// Extractor parses and extracts WF assets.
type Extractor struct {
	config  *ExtractorConfig
	paths   map[string]bool
	parsers []parser
}

// NewExtractor creates a new extractor with the supplied configuration.
// If the configuration is nil, use `DefaultExtractorConfig()`.
func NewExtractor(config *ExtractorConfig) (*Extractor, error) {
	def := DefaultExtractorConfig()
	if def == nil {
		return nil, fmt.Errorf("default configuration is nil")
	}

	if config == nil {
		config = def
	}
	if config.Workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		config.Workdir = wd
	}
	if config.Concurrency == 0 {
		config.Concurrency = 5
	}

	return &Extractor{
		config:  config,
		paths:   make(map[string]bool),
		parsers: []parser{&orderedmapParser{}, &actionDSLAMF3Parser{}},
	}, nil
}

func (extractor *Extractor) extract() error {
	paths, err := getInitialFilePaths()
	if err != nil {
		return err
	}
	items := []*concurrency.Item[*extractParams, [][]byte]{{Output: paths}}

	for {
		var newItems []*concurrency.Item[*extractParams, [][]byte]
		for _, i := range items {
			if i.Output != nil {
				newPaths := i.Output
				for _, np := range newPaths {
					if !extractor.paths[string(np)] {
						extractor.paths[string(np)] = true

						newItems = append(newItems, &concurrency.Item[*extractParams, [][]byte]{
							Data: &extractParams{
								path:    string(np),
								parsers: extractor.parsers,
								config:  extractor.config,
							},
							Output: nil,
							Err:    nil,
						})
					}
				}
			}
		}
		if len(newItems) == 0 {
			break
		}

		items = newItems
		err = concurrency.Execute[*extractParams, [][]byte](extractPath, items, extractor.config.Concurrency)
		if err != nil {
			return err
		}
	}
	return nil
}

// ExtractAssets extracts assets from downloaded files.
func (extractor *Extractor) ExtractAssets() error {
	log.Println("Extracting assets")
	return extractor.extract()
}

type extractParams struct {
	path    string
	parsers []parser
	config  *ExtractorConfig
}

func extractPath(i *concurrency.Item[*extractParams, [][]byte]) ([][]byte, error) {
	var output [][][]byte
	for _, p := range i.Data.parsers {
		o, err := extractFile(i.Data.path, p, i.Data.config)
		if err != nil {
			return nil, err
		}
		output = append(output, o)
	}
	return encoding.Flatten(output), nil
}

func extractFile(path string, p parser, config *ExtractorConfig) ([][]byte, error) {
	src, err := p.getSrc(path, config)
	if err != nil {
		return nil, err
	}
	dest, err := p.getDest(path, config)
	if err != nil {
		return nil, err
	}

	srcFile, err := os.Open(src)
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
	data, err = p.parse(data, config)
	if err != nil {
		return nil, err
	}

	os.MkdirAll(filepath.Dir(dest), 0755)
	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
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

	return p.output(data, config)
}
