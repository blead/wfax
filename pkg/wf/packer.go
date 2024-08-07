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
	"strings"

	"github.com/blead/wfax/pkg/concurrency"
)

const (
	defaultEntitiesList = "entities.csv"
)

// PackerConfig is the configuration for the packer.
type PackerConfig struct {
	SrcPath     string
	DestPath    string
	Entities    string
	Concurrency int
}

// DefaultPackerConfig generates a default configuration.
func DefaultPackerConfig() *PackerConfig {
	return &PackerConfig{
		SrcPath:     "",
		DestPath:    "",
		Entities:    "",
		Concurrency: 5,
	}
}

// Packer unparses and packs WF assets.
type Packer struct {
	config  *PackerConfig
	parsers []parser
}

type AssetDirKind int

// Enum values for AssetDirKind.
const (
	UPLOAD AssetDirKind = iota
	MEDIUM_UPLOAD
	ANDROID_UPLOAD
	ANDROID_MEDIUM_UPLOAD
)

type PathToAssetDirKindMapping struct {
	Prefix string
	Kind   AssetDirKind
}

// NewPacker creates a new packer with the supplied configuration.
// If the configuration is nil, use DefaultPackerConfig.
func NewPacker(config *PackerConfig) (*Packer, error) {
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

	if config.Entities == "" || config.Entities == "." {
		config.Entities = filepath.Join(config.DestPath, defaultEntitiesList)
	}

	if config.Concurrency == 0 {
		config.Concurrency = 5
	}

	parsers := []parser{
		// &amf3Parser{ext: ".action.dsl"},
		// &esdlParser{&amf3Parser{ext: ".esdl"}},
		&pngParser{},
		&orderedmapParser{}, // needs to be last because of ambiguous file extension
	}

	return &Packer{config: config, parsers: parsers}, nil
}

func (packer *Packer) readEntities() (map[string]AssetDirKind, error) {
	f, err := os.Open(packer.config.Entities)
	if err != nil {
		// return nil if the EntitiesList does not exist.
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("readEntities: open error, path=%s, %w", packer.config.Entities, err)
	}
	defer f.Close()

	pathToAssetDirKind := [4]PathToAssetDirKindMapping{
		{"production/upload/", UPLOAD},
		{"production/medium_upload/", MEDIUM_UPLOAD},
		{"production/android_upload/", ANDROID_UPLOAD},
		{"production/android_medium_upload/", ANDROID_MEDIUM_UPLOAD},
	}

	el := make(map[string]AssetDirKind)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		split := strings.Split(scanner.Text(), ",")
		path := split[0]

		for _, mapping := range pathToAssetDirKind {
			if strings.HasPrefix(path, mapping.Prefix) {
				el[strings.Replace(strings.TrimPrefix(path, mapping.Prefix), "/", "", 1)] = mapping.Kind
				break
			}
		}
	}

	return el, nil
}

func packFile(src string, p parser, config *PackerConfig, entities *map[string]AssetDirKind) (bool, error) {
	path, found := p.matchDest(src, config)
	if !found {
		return false, nil
	}

	dest, err := p.getSrc(path, &ExtractorConfig{SrcPath: config.DestPath})
	if err != nil {
		return false, err
	}

	hash := strings.Replace(strings.TrimPrefix(dest, filepath.Join(config.DestPath, dumpAssetDir)), "\\", "", 2)
	kind, ok := (*entities)[hash]
	if ok {
		assetDirKindToString := [4]string{
			"upload",
			"medium_upload",
			"android_upload",
			"android_medium_upload",
		}
		dest = filepath.Join(config.DestPath, assetDirKindToString[kind], hash[0:2], hash[2:])
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return false, fmt.Errorf("packFile: src open error, src=%s, dest=%s, %w", src, dest, err)
	}
	defer srcFile.Close()

	data, err := io.ReadAll(srcFile)
	if err != nil {
		return false, fmt.Errorf("packFile: src read error, src=%s, dest=%s, %w", src, dest, err)
	}

	data, err = p.unparse(data, config)
	if err != nil {
		return false, fmt.Errorf("packFile: src unparse error, src=%s, dest=%s, %w", src, dest, err)
	}

	os.MkdirAll(filepath.Dir(dest), 0777)
	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return false, fmt.Errorf("packFile: dest open error, src=%s, dest=%s, %w", src, dest, err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("packFile: dest write error, src=%s, dest=%s, %w", src, dest, err)
	}

	return true, nil
}

type packParams struct {
	path     string
	parsers  []parser
	config   *PackerConfig
	entities *map[string]AssetDirKind
}

func packPath(i *concurrency.Item[*packParams, []string]) ([]string, error) {
	f, err := os.Open(i.Data.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fileInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// f is dir: return paths inside
	if fileInfo.IsDir() {
		dirnames, err := f.Readdirnames(0)
		if err != nil {
			return nil, err
		}

		var paths []string
		for _, dn := range dirnames {
			paths = append(paths, filepath.Join(i.Data.path, dn))
		}

		return paths, nil
	}

	// f is file: pack the file
	for _, p := range i.Data.parsers {
		found, err := packFile(i.Data.path, p, i.Data.config, i.Data.entities)
		if err != nil {
			return nil, err
		}
		if found {
			break
		}
	}
	return nil, nil
}

func (packer *Packer) pack() error {
	err := os.MkdirAll(packer.config.DestPath, 0777)
	if err != nil {
		return err
	}

	entities, err := packer.readEntities()
	if err != nil {
		return err
	}

	f, err := os.Open(packer.config.SrcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	dirnames, err := f.Readdirnames(0)
	if err != nil {
		return err
	}

	var paths []string
	for _, dn := range dirnames {
		paths = append(paths, filepath.Join(packer.config.SrcPath, dn))
	}

	items := []*concurrency.Item[*packParams, []string]{{Output: paths}}

	return concurrency.Dispatcher(
		func(i *concurrency.Item[*packParams, []string]) ([]*concurrency.Item[*packParams, []string], error) {
			var output []*concurrency.Item[*packParams, []string]
			if i.Output != nil {
				for _, p := range i.Output {
					output = append(output, &concurrency.Item[*packParams, []string]{
						Data: &packParams{
							path:     p,
							parsers:  packer.parsers,
							config:   packer.config,
							entities: &entities,
						},
						Output: nil,
						Err:    nil,
					})
				}
			}
			return output, nil
		},
		packPath,
		items,
		packer.config.Concurrency,
	)
}

// PackAssets packs extracted files back into game asset format.
func (packer *Packer) PackAssets() error {
	log.Println("[INFO] Packing assets")
	return packer.pack()
}
