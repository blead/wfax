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

func (_ *orderedmapParser) getSrc(path string, config *ExtractorConfig) (string, error) {
	src, err := sha1Digest(toMasterTablePath(string(path)), digestSalt)
	if err != nil {
		return "", err
	}
	return filepath.Join(config.SrcPath, dumpAssetDir, src[0:2], src[2:]), nil
}

func (_ *orderedmapParser) getDest(path string, config *ExtractorConfig) (string, error) {
	return filepath.Join(config.DestPath, outputOrderedMapDir, addExt(filepath.FromSlash(path), ".json")), nil
}

func (_ *orderedmapParser) parse(raw []byte, config *ExtractorConfig) ([]byte, error) {
	return encoding.OrderedmapToJSON(raw, config.FlattenCSV)
}

func (_ *orderedmapParser) output(raw []byte, config *ExtractorConfig) ([][]byte, error) {
	return findAllPaths(raw)
}

type actionDSLAMF3Parser struct{}

func (_ *actionDSLAMF3Parser) getSrc(path string, config *ExtractorConfig) (string, error) {
	src, err := sha1Digest(addExt(string(path), ".action.dsl.amf3.deflate"), digestSalt)
	if err != nil {
		return "", err
	}
	return filepath.Join(config.SrcPath, dumpAssetDir, src[0:2], src[2:]), nil
}

func (_ *actionDSLAMF3Parser) getDest(path string, config *ExtractorConfig) (string, error) {
	return filepath.Join(config.DestPath, outputAssetsDir, addExt(filepath.FromSlash(path), ".action.dsl.json")), nil
}

func (_ *actionDSLAMF3Parser) parse(raw []byte, config *ExtractorConfig) ([]byte, error) {
	return encoding.Amf3ToJSON(raw)
}

func (_ *actionDSLAMF3Parser) output(raw []byte, config *ExtractorConfig) ([][]byte, error) {
	return findAllPaths(raw)
}
