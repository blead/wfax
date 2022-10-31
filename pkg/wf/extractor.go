package wf

import (
	"fmt"
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
	config *ExtractorConfig
	paths  map[string]bool
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
		config: config,
		paths:  make(map[string]bool),
	}, nil
}

func (extractor *Extractor) unmarkPaths() {
	for path := range extractor.paths {
		extractor.paths[path] = false
	}
}

func (extractor *Extractor) pathsByteErr() ([][]byte, error) {
	var paths [][]byte
	for path := range extractor.paths {
		paths = append(paths, []byte(path))
	}
	return paths, nil
}

func (extractor *Extractor) extract(getPaths func() ([][]byte, error), getItem func([]byte) (*concurrency.Item, error)) error {
	extractor.unmarkPaths()
	paths, err := getPaths()
	if err != nil {
		return err
	}
	items := []*concurrency.Item{{Output: paths}}

	for {
		var newItems []*concurrency.Item
		for _, i := range items {
			if i.Output != nil {
				newPaths, ok := i.Output.([][]byte)
				if !ok {
					return fmt.Errorf("unable to cast item.Output to [][]byte: %v", *i)
				}

				for _, np := range newPaths {
					if !extractor.paths[string(np)] {
						extractor.paths[string(np)] = true

						newItem, err := getItem(np)
						if err != nil {
							return err
						}
						newItems = append(newItems, newItem)
					}
				}
			}
		}
		if len(newItems) == 0 {
			break
		}

		items = newItems
		err = concurrency.Execute(extractFile, items, extractor.config.Concurrency)
		if err != nil {
			return err
		}
	}
	return nil
}

func (extractor *Extractor) extractMasterTable() error {
	return extractor.extract(
		getInitialFilePaths,
		func(path []byte) (*concurrency.Item, error) {
			src, err := sha1Digest(toMasterTablePath(string(path)), digestSalt)
			if err != nil {
				return nil, err
			}
			dest := addExt(filepath.FromSlash(string(path)), ".json")

			return &concurrency.Item{
				Data: extractParams{
					src:     filepath.Join(extractor.config.Workdir, dumpDir, dumpAssetDir, src[0:2], src[2:]),
					dest:    filepath.Join(extractor.config.Workdir, outputDir, outputOrderedMapDir, dest),
					extract: encoding.OrderedmapToJSON,
					output:  findAllPaths,
				},
				Output: nil,
				Err:    nil,
			}, nil
		},
	)
}

func (extractor *Extractor) extractActionDSLAMF3() error {
	return extractor.extract(
		extractor.pathsByteErr,
		func(path []byte) (*concurrency.Item, error) {
			src, err := sha1Digest(addExt(string(path), ".action.dsl.amf3.deflate"), digestSalt)
			if err != nil {
				return nil, err
			}
			dest := addExt(filepath.FromSlash(string(path)), ".action.dsl.json")

			return &concurrency.Item{
				Data: extractParams{
					src:     filepath.Join(extractor.config.Workdir, dumpDir, dumpAssetDir, src[0:2], src[2:]),
					dest:    filepath.Join(extractor.config.Workdir, outputDir, outputAssetsDir, dest),
					extract: encoding.Amf3ToJSON,
					output:  findAllPaths,
				},
				Output: nil,
				Err:    nil,
			}, nil
		},
	)
}

// ExtractAssets extracts assets from downloaded files.
func (extractor *Extractor) ExtractAssets() error {
	log.Println("Extracting master tables")
	err := extractor.extractMasterTable()
	if err != nil {
		return err
	}

	log.Println("Extracting Action DSL files")
	return extractor.extractActionDSLAMF3()
}
