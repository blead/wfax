package wf

import (
	"path/filepath"

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
