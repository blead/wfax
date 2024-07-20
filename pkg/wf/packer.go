package wf

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/blead/wfax/pkg/concurrency"
)

type Packer struct {
	config  *ExtractorConfig
	parsers []parser
}

func DefaultPackerConfig() *ExtractorConfig {
	return &ExtractorConfig{
		SrcPath:        "",
		DestPath:       "",
		PathList:       "",
		NoDefaultPaths: false,
		Concurrency:    5,
		Indent:         0,
		FlattenCSV:     false,
		Eliyabot:       false,
	}
}

func NewPacker(config *ExtractorConfig) (*Packer, error) {
	def := DefaultPackerConfig()
	if def == nil {
		return nil, fmt.Errorf("NewPacker: default configuration is nil")
	}

	if config == nil {
		config = def
	}

	if config.SrcPath == "" || config.SrcPath == "." {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		config.SrcPath = wd
	}
	config.SrcPath = filepath.Clean(config.SrcPath)
	if config.DestPath == "" || config.DestPath == "." {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		config.DestPath = wd
	}
	config.DestPath = filepath.Clean(config.DestPath)
	if config.PathList == "" || config.PathList == "." {
		config.PathList = filepath.Join(config.DestPath, defaultPathList)
	}
	config.PathList = filepath.Clean(config.PathList)

	if config.Concurrency == 0 {
		config.Concurrency = 5
	}

	parsers := []parser{
		&orderedmapParser{},
	}

	return &Packer{
		config:  config,
		parsers: parsers,
	}, nil
}

func (packer *Packer) readPathList() ([]string, error) {
	f, err := os.Open(packer.config.PathList)
	if err != nil {
		// return nil if pathlist does not exist
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("readPathList: open error, path=%s, %w", packer.config.PathList, err)
	}
	defer f.Close()

	var pl []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		pl = append(pl, scanner.Text())
	}
	return pl, scanner.Err()
}

func (extractor *Packer) getInitialPaths() ([][]byte, error) {
	paths := map[string]struct{}{}
	if len(extractor.config.PathList) > 0 {
		pl, err := extractor.readPathList()
		if err != nil {
			return nil, err
		}

		for _, p := range pl {
			paths[p] = struct{}{}
		}
	}

	var output [][]byte
	for p := range paths {
		output = append(output, []byte(p))
	}
	return output, nil
}

func (packer *Packer) pack() error {
	err := os.MkdirAll(packer.config.DestPath, 0777)
	if err != nil {
		return err
	}

	paths, err := packer.getInitialPaths()
	if err != nil {
		return err
	}
	items := []*concurrency.Item[*extractParams, [][]byte]{{Output: paths}}
	seenPaths := map[string]bool{}

	err = concurrency.Dispatcher(
		func(i *concurrency.Item[*extractParams, [][]byte]) ([]*concurrency.Item[*extractParams, [][]byte], error) {
			var output []*concurrency.Item[*extractParams, [][]byte]
			if i.Output != nil {
				for _, p := range i.Output {
					if !seenPaths[string(p)] {
						seenPaths[string(p)] = true
						output = append(output, &concurrency.Item[*extractParams, [][]byte]{
							Data: &extractParams{
								path:    string(p),
								parsers: packer.parsers,
								config:  packer.config,
							},
							Output: nil,
							Err:    nil,
						})
					}
				}
			}
			if len(output) > 0 {
				fmt.Println(len(output))
			}
			return output, nil
		},
		extractPath,
		items,
		packer.config.Concurrency,
	)

	return err
}

func (packer *Packer) PackAssets() error {
	log.Println("[INFO] Packing assets")
	return packer.pack()
}
