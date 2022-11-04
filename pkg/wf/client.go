package wf

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Jeffail/gabs/v2"
	"github.com/blead/wfax/pkg/concurrency"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/tinylib/msgp/msgp"
)

const (
	defaultVersion = "0.0.0"
	apiHostJp      = "https://api.worldflipper.jp/latest/api/index.php"
	apiAssetJp     = "https://api.worldflipper.jp/latest/api/index.php/gacha/exec"
)

// AssetListMode specifies whether to retrive full asset list or diff only.
type AssetListMode int

// Enum values for AssetListMode.
const (
	FullAssets AssetListMode = iota
	DiffAssets
)

// ClientConfig is the configuration for the client.
type ClientConfig struct {
	Version     string
	Mode        AssetListMode
	Workdir     string
	Concurrency int
}

// DefaultClientConfig generates a default configuration.
func DefaultClientConfig() *ClientConfig {
	config := &ClientConfig{
		Version:     defaultVersion,
		Mode:        FullAssets,
		Workdir:     "",
		Concurrency: 5,
	}

	return config
}

// Client communicating with WF API.
type Client struct {
	config *ClientConfig
	client *retryablehttp.Client
	header *http.Header
}

// NewClient creates a new client with the supplied configuration.
// If the configuration is nil, use `DefaultClientConfig()`.
func NewClient(config *ClientConfig) (*Client, error) {
	def := DefaultClientConfig()
	if def == nil {
		return nil, fmt.Errorf("default configuration is nil")
	}

	if config == nil {
		config = def
	}
	if config.Version == "" {
		config.Version = defaultVersion
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

	return &Client{
		config: config,
		client: retryablehttp.NewClient(),
		header: clientHeader(config.Version),
	}, nil
}

func clientHeader(version string) *http.Header {
	return &http.Header{
		"User-Agent": {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.0.0 Safari/537.36"},
		"Accept":     {"gzip, deflate, br"},
		"RES_VER":    {version},
	}
}

func (client *Client) fetchMetadata() ([]byte, error) {
	req, err := retryablehttp.NewRequest("GET", apiAssetJp, nil)
	if err != nil {
		return nil, err
	}

	req.Header = *client.header
	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			panic(err)
		}
	}()

	var output bytes.Buffer
	_, err = msgp.CopyToJSON(&output, base64.NewDecoder(base64.StdEncoding, resp.Body))
	if err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}

type assetMetadata struct {
	location string
	size     int
	sha256   string
}

func (client *Client) parseMetadata(json []byte) (string, []*assetMetadata, error) {
	jsonParsed, err := gabs.ParseJSON(json)
	if err != nil {
		return "", nil, err
	}
	if !jsonParsed.ExistsP("data.info") {
		return "", []*assetMetadata{}, nil
	}

	version, ok := jsonParsed.Path("data.info.eventual_target_asset_version").Data().(string)
	if !ok {
		return "", nil, fmt.Errorf("unable to parse latest version number")
	}

	var assets []*assetMetadata
	if client.config.Mode == FullAssets {
		for _, child := range jsonParsed.Path("data.full.archive").Children() {
			assets = append(assets, &assetMetadata{
				location: child.Path("location").Data().(string),
				size:     int(child.Path("size").Data().(float64)),
				sha256:   child.Path("sha256").Data().(string),
			})
		}
	} else {
		for _, group := range jsonParsed.Search("data", "diff", "*", "archive").Children() {
			for _, child := range group.Children() {
				assets = append(assets, &assetMetadata{
					location: child.Path("location").Data().(string),
					size:     int(child.Path("size").Data().(float64)),
					sha256:   child.Path("sha256").Data().(string),
				})
			}
		}
	}

	return version, assets, nil
}

func (client *Client) downloadAndExtract(i *concurrency.Item[*assetMetadata, any]) (any, error) {
	a := i.Data
	resp, err := retryablehttp.Get(a.location)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			panic(err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Compare checksum
	expected, err := base64.StdEncoding.DecodeString(a.sha256)
	if err != nil {
		return nil, err
	}
	downloaded, err := sha256Checksum(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if bytes.Compare(expected, downloaded) != 0 {
		return nil, fmt.Errorf("sha256 mismatch, expected: %x, downloaded: %x", expected, downloaded)
	}

	// extract zip
	err = unzip(
		bytes.NewReader(body),
		int64(len(body)),
		filepath.Join(client.config.Workdir, dumpDir),
		func(path string) string {
			pattern := regexp.MustCompile(`production/[^/]*`)
			return filepath.FromSlash(pattern.ReplaceAllLiteralString(filepath.ToSlash(path), dumpAssetDir))
		})
	return nil, err
}

func (client *Client) fetch(assets []*assetMetadata) error {
	var items []*concurrency.Item[*assetMetadata, any]
	for _, a := range assets {
		items = append(items, &concurrency.Item[*assetMetadata, any]{
			Data:   a,
			Output: nil,
			Err:    nil,
		})
	}

	return concurrency.Execute(client.downloadAndExtract, items, client.config.Concurrency)
}

// FetchAssetsFromAPI fetches metadata from API then download and extract the assets archives.
func (client *Client) FetchAssetsFromAPI() error {
	log.Println("Fetching asset metadata")
	metadata, err := client.fetchMetadata()
	if err != nil {
		return err
	}

	latestVersion, assets, err := client.parseMetadata(metadata)
	if err != nil {
		return err
	}
	if len(assets) == 0 {
		log.Println("no new assets for version " + client.config.Version)
		fmt.Println(latestVersion)
	}

	log.Println("fetching assets for version " + latestVersion)
	err = client.fetch(assets)
	if err != nil {
		return err
	}

	fmt.Println(latestVersion)
	return nil
}
