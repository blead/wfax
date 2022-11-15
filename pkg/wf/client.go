package wf

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
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
	dumpAssetDir   = "upload"
)

var ErrNoNewAssets = errors.New("no new assets")

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
	tmpDir string
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
	if config.Workdir == "" || config.Workdir == "." {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		config.Workdir = wd
	}
	if config.Concurrency == 0 {
		config.Concurrency = 5
	}

	client := retryablehttp.NewClient()
	client.Logger = log.Default()

	return &Client{
		config: config,
		client: client,
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
	defer resp.Body.Close()

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

func (client *Client) download(i *concurrency.Item[*assetMetadata, any]) (any, error) {
	a := i.Data
	resp, err := client.client.Get(a.location)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

	dest := filepath.Join(client.tmpDir, path.Base(a.location))
	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, fmt.Errorf("open error, path=%s, %w", dest, err)
	}
	defer func() {
		err := destFile.Close()
		if err != nil {
			log.Fatal(fmt.Errorf("close error, path=%s, %w", dest, err))
		}
	}()

	_, err = io.Copy(destFile, bytes.NewReader(body))
	return nil, err
}

func (client *Client) extract(i *concurrency.Item[*assetMetadata, any]) (any, error) {
	a := i.Data
	src := filepath.Join(client.tmpDir, path.Base(a.location))
	srcFile, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("open error, src=%s, %w", src, err)
	}
	defer srcFile.Close()

	zdata, err := io.ReadAll(srcFile)
	if err != nil {
		return nil, err
	}

	err = unzip(
		bytes.NewReader(zdata),
		int64(len(zdata)),
		client.config.Workdir,
		func(path string) string {
			pattern := regexp.MustCompile(`production/[^/]*`)
			return filepath.FromSlash(pattern.ReplaceAllLiteralString(filepath.ToSlash(path), dumpAssetDir))
		})
	return nil, err
}

func (client *Client) downloadAndExtract(assets []*assetMetadata) error {
	var items []*concurrency.Item[*assetMetadata, any]
	for _, a := range assets {
		items = append(items, &concurrency.Item[*assetMetadata, any]{
			Data:   a,
			Output: nil,
			Err:    nil,
		})
	}

	err := os.MkdirAll(client.config.Workdir, 0777)
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(client.config.Workdir, "fetchtmp")
	if err != nil {
		return err
	}
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			log.Fatal(fmt.Errorf("remove error, path=%s, %w", tmpDir, err))
		}
	}()

	client.tmpDir = tmpDir
	con := client.config.Concurrency
	if len(items) < con {
		con = len(items)
	}

	err = concurrency.Execute(client.download, items, con)
	if err != nil {
		return err
	}

	return concurrency.Execute(client.extract, items, con)
}

// FetchAssetsFromAPI fetches metadata from API then download and extract the assets archives.
func (client *Client) FetchAssetsFromAPI() error {
	log.Println("[INFO] Fetching asset metadata, clientVersion=" + client.config.Version)
	metadata, err := client.fetchMetadata()
	if err != nil {
		return err
	}

	latestVersion, assets, err := client.parseMetadata(metadata)
	if err != nil {
		return err
	}
	if len(assets) == 0 {
		log.Println("[INFO] No new assets")
		return ErrNoNewAssets
	}

	log.Printf("[INFO] Fetching assets, clientVersion=%s, latestVersion=%s\n", client.config.Version, latestVersion)
	err = client.downloadAndExtract(assets)
	if err != nil {
		return err
	}

	fmt.Println(latestVersion)
	return nil
}
