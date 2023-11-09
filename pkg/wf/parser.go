package wf

import (
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/Jeffail/gabs/v2"
	"github.com/blead/wfax/pkg/encoding"
)

type parser interface {
	getSrc(string, *ExtractorConfig) (string, error)
	getDest(string, *ExtractorConfig) (string, error)
	parse([]byte, *ExtractorConfig) ([]byte, error)
	output([]byte, *ExtractorConfig) ([][]byte, error)
}

type orderedmapParser struct{}

func (*orderedmapParser) getSrc(path string, config *ExtractorConfig) (string, error) {
	src, err := sha1Digest(toMasterTablePath(string(path)), digestSalt)
	if err != nil {
		return "", err
	}
	return filepath.Join(config.SrcPath, dumpAssetDir, src[0:2], src[2:]), nil
}

func (*orderedmapParser) getDest(path string, config *ExtractorConfig) (string, error) {
	return addExt(filepath.Join(config.DestPath, outputOrderedMapDir, filepath.FromSlash(path)), ".json"), nil
}

func (*orderedmapParser) parse(raw []byte, config *ExtractorConfig) ([]byte, error) {
	return encoding.OrderedmapToJSON(raw, config.Indent, config.FlattenCSV)
}

func (*orderedmapParser) output(raw []byte, config *ExtractorConfig) ([][]byte, error) {
	return findAllPaths(raw)
}

type amf3Parser struct {
	ext string
}

func (parser *amf3Parser) getSrc(path string, config *ExtractorConfig) (string, error) {
	src, err := sha1Digest(addExt(string(path), parser.ext+".amf3.deflate"), digestSalt)
	if err != nil {
		return "", err
	}
	return filepath.Join(config.SrcPath, dumpAssetDir, src[0:2], src[2:]), nil
}

func (parser *amf3Parser) getDest(path string, config *ExtractorConfig) (string, error) {
	return addExt(filepath.Join(config.DestPath, outputAssetsDir, filepath.FromSlash(path)), parser.ext+".json"), nil
}

func (*amf3Parser) parse(raw []byte, config *ExtractorConfig) ([]byte, error) {
	return encoding.Amf3ToJSON(raw, config.Indent)
}

func (*amf3Parser) output(raw []byte, config *ExtractorConfig) ([][]byte, error) {
	return findAllPaths(raw)
}

type esdlParser struct {
	*amf3Parser
}

func (*esdlParser) searchESDLPaths(jsonParsed *gabs.Container, s []string) []string {
	var paths []string
	filePaths, err := jsonParsed.Search(s...).Flatten()
	if err == nil {
		for _, fp := range filePaths {
			path, ok := fp.(string)
			if ok {
				paths = append(paths, path)
			}
		}
	}

	return paths
}

func (parser *esdlParser) output(raw []byte, config *ExtractorConfig) ([][]byte, error) {
	jsonParsed, err := gabs.ParseJSON(raw)
	if err != nil {
		return findAllPaths(raw)
	}

	// bH = basePath
	basePath, ok := jsonParsed.Path("bH").Data().(string)
	if !ok {
		return findAllPaths(raw)
	}

	searches := [][]string{
		// au = forms, g = states, i = actions, b = filePath
		{"au", "*", "g", "*", "i", "*", "b"},
		// m = deadActions
		{"au", "*", "m", "*", "b"},
		// i = stunActions
		{"au", "*", "i", "*", "b"},
		// k = winceActions
		{"au", "*", "k", "*", "b"},
	}
	var paths [][]byte

	for _, s := range searches {
		for _, path := range parser.searchESDLPaths(jsonParsed, s) {
			paths = append(paths, []byte(basePath+path))
		}
	}

	// h = watches, i = content
	for _, form := range jsonParsed.Search("au", "*", "h", "*", "i").Children() {
		for _, watch := range form.Children() {
			// enum index "T4" only
			index, ok := watch.Search("0").Data().(string)
			if ok && index == "T4" {
				for _, path := range parser.searchESDLPaths(watch, []string{"1"}) {
					paths = append(paths, []byte(basePath+path))
				}
			}
		}
	}

	// bx = type, enum index "T1" only
	typeContainer := jsonParsed.Search("bx")
	index, ok := typeContainer.Search("0").Data().(string)
	if ok && index == "T1" {
		// g = pre_action_path
		for _, path := range parser.searchESDLPaths(typeContainer, []string{"1", "g"}) {
			paths = append(paths, []byte(basePath+path))
		}
	}

	return paths, nil
}

type pngParser struct {
}

func (*pngParser) getSrc(path string, config *ExtractorConfig) (string, error) {
	src, err := sha1Digest(addExt(string(path), ".png"), digestSalt)
	if err != nil {
		return "", err
	}
	return filepath.Join(config.SrcPath, dumpAssetDir, src[0:2], src[2:]), nil
}

func (*pngParser) getDest(path string, config *ExtractorConfig) (string, error) {
	return addExt(filepath.Join(config.DestPath, outputAssetsDir, filepath.FromSlash(path)), ".png"), nil
}

func (*pngParser) parse(raw []byte, config *ExtractorConfig) ([]byte, error) {
	// p n g
	if raw[1] != 0x70 || raw[2] != 0x6e || raw[3] != 0x67 {
		return nil, fmt.Errorf("pngParser: png header mismatch, expected: 706e67, found: %x", hex.EncodeToString(raw[1:4]))
	}

	raw[1] = 0x50
	raw[2] = 0x4e
	raw[3] = 0x47

	return raw, nil
}

func (*pngParser) output(raw []byte, config *ExtractorConfig) ([][]byte, error) {
	return [][]byte{}, nil
}

type charPngParser struct {
	*pngParser
	srcTemplate  string
	destTemplate string
	width        int
	height       int
}

func (parser *charPngParser) getSrc(path string, config *ExtractorConfig) (string, error) {
	return parser.pngParser.getSrc(fmt.Sprintf(parser.srcTemplate, path), config)
}

func (parser *charPngParser) getDest(path string, config *ExtractorConfig) (string, error) {
	return parser.pngParser.getDest(fmt.Sprintf(parser.destTemplate, path), config)
}

func (parser *charPngParser) parse(raw []byte, config *ExtractorConfig) ([]byte, error) {
	src, err := parser.pngParser.parse(raw, config)
	if err != nil {
		return nil, err
	}

	return encoding.FitPNG(src, parser.width, parser.height)
}
