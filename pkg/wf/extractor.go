package wf

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blead/wfax/assets"
	"github.com/blead/wfax/pkg/concurrency"
	"github.com/blead/wfax/pkg/encoding"
)

const (
	digestSalt          = "K6R9T9Hz22OpeIGEWB0ui6c6PYFQnJGy"
	outputAssetsDir     = "assets"
	outputOrderedMapDir = "orderedmap"
	defaultPathList     = ".pathlist"
)

// ExtractorConfig is the configuration for the extractor.
type ExtractorConfig struct {
	SrcPath     string
	DestPath    string
	PathList    string
	Concurrency int
	Indent      int
	FlattenCSV  bool
}

// DefaultExtractorConfig generates a default configuration.
func DefaultExtractorConfig() *ExtractorConfig {
	return &ExtractorConfig{
		SrcPath:     "",
		DestPath:    "",
		PathList:    "",
		Concurrency: 5,
		Indent:      0,
		FlattenCSV:  false,
	}
}

// Extractor parses and extracts WF assets.
type Extractor struct {
	config  *ExtractorConfig
	parsers []parser
}

// NewExtractor creates a new extractor with the supplied configuration.
// If the configuration is nil, use DefaultExtractorConfig.
func NewExtractor(config *ExtractorConfig) (*Extractor, error) {
	def := DefaultExtractorConfig()
	if def == nil {
		return nil, fmt.Errorf("NewExtractor: default configuration is nil")
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

	return &Extractor{
		config: config,
		parsers: []parser{
			&orderedmapParser{},
			&amf3Parser{ext: ".action.dsl"},
			&esdlParser{&amf3Parser{ext: ".esdl"}},
		},
	}, nil
}

func (extractor *Extractor) readPathList() ([]string, error) {
	f, err := os.Open(extractor.config.PathList)
	if err != nil {
		// return nil if pathlist does not exist
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("readPathList: open error, path=%s, %w", extractor.config.PathList, err)
	}
	defer f.Close()

	var pl []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		pl = append(pl, scanner.Text())
	}
	return pl, scanner.Err()
}

func (extractor *Extractor) writePathList(pl []string) error {
	sort.Strings(pl)

	f, err := os.OpenFile(extractor.config.PathList, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("writePathList: open error, path=%s, %w", extractor.config.PathList, err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Fatal(fmt.Errorf("writePathList: close error, path=%s, %w", extractor.config.PathList, err))
		}
	}()

	writer := bufio.NewWriter(f)
	for _, p := range pl {
		_, err = writer.WriteString(p + "\n")
		if err != nil {
			return fmt.Errorf("writePathList: write error, path=%s, %w", extractor.config.PathList, err)
		}
	}
	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("writePathList: flush error, path=%s, %w", extractor.config.PathList, err)
	}

	return err
}

func (extractor *Extractor) getInitialPaths() ([][]byte, error) {
	paths := map[string]struct{}{}
	for _, p := range strings.Split(assets.PathList, "\n") {
		paths[p] = struct{}{}
	}
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
		// return nil if srcFile does not exist
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("extractFile: src open error, src=%s, dest=%s, %w", src, dest, err)
	}
	defer srcFile.Close()

	data, err := io.ReadAll(srcFile)
	if err != nil {
		return nil, fmt.Errorf("extractFile: src read error, src=%s, dest=%s, %w", src, dest, err)
	}
	data, err = p.parse(data, config)
	if err != nil {
		return nil, fmt.Errorf("extractFile: src parse error, src=%s, dest=%s, %w", src, dest, err)
	}

	os.MkdirAll(filepath.Dir(dest), 0777)
	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, fmt.Errorf("extractFile: dest open error, src=%s, dest=%s, %w", src, dest, err)
	}
	defer func() {
		err := destFile.Close()
		if err != nil {
			log.Fatal(fmt.Errorf("extractFile: dest close error, src=%s, dest=%s, %w", src, dest, err))
		}
	}()

	_, err = io.Copy(destFile, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("extractFile: dest write error, src=%s, dest=%s, %w", src, dest, err)
	}

	return p.output(data, config)
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
		// o = nil if file does not exist with format p
		if o != nil {
			output = append(output, o)
		}
	}
	return encoding.Flatten(output), nil
}

func (extractor *Extractor) extract() error {
	err := os.MkdirAll(extractor.config.DestPath, 0777)
	if err != nil {
		return err
	}

	paths, err := extractor.getInitialPaths()
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
								parsers: extractor.parsers,
								config:  extractor.config,
							},
							Output: nil,
							Err:    nil,
						})
					}
				}
			}
			return output, nil
		},
		extractPath,
		items,
		extractor.config.Concurrency,
	)
	if err != nil {
		return err
	}

	var pathList []string
	for p := range seenPaths {
		if len(p) > 0 {
			pathList = append(pathList, p)
		}
	}
	return extractor.writePathList(pathList)
}

// ExtractAssets extracts assets from downloaded files.
func (extractor *Extractor) ExtractAssets() error {
	log.Println("[INFO] Extracting assets")
	return extractor.extract()
}
