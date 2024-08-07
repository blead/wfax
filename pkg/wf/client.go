package wf

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/blead/wfax/pkg/concurrency"
	"github.com/hashicorp/go-cleanhttp"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/tinylib/msgp/msgp"
)

const (
	defaultVersion   = "0.0.0"
	dumpAssetDir     = "upload"
	apiHostJP        = "https://api.worldflipper.jp"
	apiHostGL        = "https://na.wdfp.kakaogames.com"
	apiHostKR        = "https://kr.wdfp.kakaogames.com"
	apiHostCN        = "https://shijtswygamegf.leiting.com"
	apiHostTW        = "https://wf-game.worldflipper.beanfun.com"
	apiAssetEndpoint = "/latest/api/index.php/gacha/exec"
	apiComicEndpoint = "/latest/api/index.php/comic/get_list"
	cdnAddressGL     = "http://patch.wdfp.kakaogames.com/Live/2.0.0"
	cdnAddressKR     = "http://patch.wdfp.kakaogames.com/Live/2.0.0"
	viewerIDJP       = 938889939
	viewerIDGL       = 752309378
	viewerIDKR       = 885870369
	viewerIDCN       = 554279419
	viewerIDTW       = 714420616
)

var ErrNoNewAssets = errors.New("no new assets")

// AssetListMode specifies whether to retrive full asset list or diff only.
type AssetListMode int

// Enum values for AssetListMode.
const (
	FullAssets AssetListMode = iota
	DiffAssets
)

type ServiceRegion int

// Enum values for ServiceRegion.
const (
	RegionJP ServiceRegion = iota
	RegionGL
	RegionKR
	RegionCN
	RegionTW
	RegionTH
)

func getAPIEndpoint(region ServiceRegion, endpoint string) string {
	if endpoint == "" {
		endpoint = apiAssetEndpoint
	}
	switch region {
	case RegionJP:
		return apiHostJP + endpoint
	case RegionGL, RegionTH:
		return apiHostGL + endpoint
	case RegionKR:
		return apiHostKR + endpoint
	case RegionCN:
		return apiHostCN + endpoint
	case RegionTW:
		return apiHostTW + endpoint
	}
	log.Printf("[WARN] getAPIEndpoint: unknown region, using default (JP), region=%v, endpoint=%v\n", region, endpoint)
	return apiHostJP + endpoint
}

func getCDNAddress(region ServiceRegion) string {
	switch region {
	case RegionGL, RegionTH:
		return cdnAddressGL
	case RegionKR:
		return cdnAddressKR
	default:
		return ""
	}
}

func replaceCDNAddress(location string, cdnAddress string) string {
	if cdnAddress != "" {
		return strings.ReplaceAll(location, "{$cdnAddress}", cdnAddress)
	}
	return location
}

func getViewerID(region ServiceRegion) int {
	switch region {
	case RegionJP:
		return viewerIDJP
	case RegionGL, RegionTH:
		return viewerIDGL
	case RegionKR:
		return viewerIDKR
	case RegionCN:
		return viewerIDCN
	case RegionTW:
		return viewerIDTW
	}
	log.Printf("[WARN] getViewerID: unknown region, using default (JP), region=%v\n", region)
	return viewerIDJP
}

// ClientConfig is the configuration for the client.
type ClientConfig struct {
	Version     string
	Mode        AssetListMode
	Workdir     string
	Concurrency int
	Region      ServiceRegion
	CustomAPI   string
	CustomCDN   string
}

// DefaultClientConfig generates a default configuration.
func DefaultClientConfig() *ClientConfig {
	config := &ClientConfig{
		Version:     defaultVersion,
		Mode:        FullAssets,
		Workdir:     "",
		Concurrency: 5,
		Region:      RegionJP,
		CustomAPI:   "",
		CustomCDN:   "",
	}

	return config
}

// Client communicating with WF API.
type Client struct {
	config     *ClientConfig
	client     *retryablehttp.Client
	header     *http.Header
	tmpDir     string
	extractMap map[string]string
}

// NewClient creates a new client with the supplied configuration.
// If the configuration is nil, use DefaultClientConfig.
func NewClient(config *ClientConfig) (*Client, error) {
	def := DefaultClientConfig()
	if def == nil {
		return nil, fmt.Errorf("NewClient: default configuration is nil")
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
	config.Workdir = filepath.Clean(config.Workdir)
	if config.Concurrency == 0 {
		config.Concurrency = 5
	}

	transport := cleanhttp.DefaultPooledTransport()
	transport.RegisterProtocol("file", http.NewFileTransport(http.Dir(".")))

	client := retryablehttp.NewClient()
	client.HTTPClient = &http.Client{Transport: transport}
	client.Logger = log.Default()

	return &Client{
		config: config,
		client: client,
		header: clientHeader(config.Version, config.Region),
	}, nil
}

func clientHeader(version string, region ServiceRegion) *http.Header {
	header := &http.Header{
		"User-Agent": {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.0.0 Safari/537.36"},
		"Accept":     {"gzip, deflate, br"},
		"RES_VER":    {version},
	}

	if region == RegionGL {
		header.Set("DEVICE_LANG", "en")
	} else if region == RegionTH {
		header.Set("DEVICE_LANG", "th")
	} else if region == RegionKR {
		header.Set("DEVICE_LANG", "ko")
	}

	return header
}

func (client *Client) fetchMsgp(req *retryablehttp.Request) ([]byte, error) {
	resp, err := client.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchMsqgp: non-2xx status code in response, status=%d, url=%s", resp.StatusCode, req.URL.String())
	}

	var output bytes.Buffer
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	_, err = msgp.CopyToJSON(&output, base64.NewDecoder(base64.StdEncoding, bytes.NewReader(body)))
	if err != nil {
		// not base64, check if body is plain json
		_, ok := err.(base64.CorruptInputError)
		if ok && json.Valid(body) {
			return body, nil
		}
		return nil, err
	}

	return output.Bytes(), nil
}

type assetMetadata struct {
	location string
	dest     string
	sha256   string
}

func (client *Client) parseMetadata(json []byte, parseAssets bool) (string, []*assetMetadata, error) {
	jsonParsed, err := gabs.ParseJSON(json)
	if err != nil {
		return "", nil, err
	}

	// make outer data object optional to support asset list file in starpoint
	if jsonParsed.ExistsP("data") {
		jsonParsed = jsonParsed.Path("data")
	}

	if !jsonParsed.ExistsP("info") {
		return "", []*assetMetadata{}, nil
	}

	version, ok := jsonParsed.Path("info.eventual_target_asset_version").Data().(string)
	if !ok {
		return "", nil, fmt.Errorf("parseMetadata: unable to parse latest version number")
	}

	var assets []*assetMetadata

	if parseAssets {
		cdnAddress := client.config.CustomCDN
		if cdnAddress == "" {
			cdnAddress = getCDNAddress(client.config.Region)
		}

		if client.config.Mode == FullAssets {
			for _, child := range jsonParsed.Path("full.archive").Children() {
				assets = append(assets, &assetMetadata{
					location: replaceCDNAddress(child.Path("location").Data().(string), cdnAddress),
					dest:     client.tmpDir,
					sha256:   child.Path("sha256").Data().(string),
				})
			}
		}
		for _, group := range jsonParsed.Search("diff", "*", "archive").Children() {
			for _, child := range group.Children() {
				assets = append(assets, &assetMetadata{
					location: replaceCDNAddress(child.Path("location").Data().(string), cdnAddress),
					dest:     client.tmpDir,
					sha256:   child.Path("sha256").Data().(string),
				})
			}
		}
	}

	return version, assets, nil
}

func modPath(path string) string {
	pattern := regexp.MustCompile(`production/[^/]*`)
	return filepath.FromSlash(pattern.ReplaceAllLiteralString(filepath.ToSlash(path), dumpAssetDir))
}

func (client *Client) download(i *concurrency.Item[*assetMetadata, []string]) ([]string, error) {
	a := i.Data
	resp, err := client.client.Get(a.location)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download: non-2xx status code in response, status=%d, url=%s", resp.StatusCode, a.location)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if a.sha256 != "" {
		// Compare checksum
		expected, err := base64.StdEncoding.DecodeString(a.sha256)
		if err != nil {
			return nil, err
		}
		downloaded, err := sha256Checksum(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(expected, downloaded) {
			return nil, fmt.Errorf("download: sha256 mismatch, expected: %x, downloaded: %x, url: %s", expected, downloaded, a.location)
		}
	}

	dest := filepath.Join(a.dest, path.Base(a.location))

	err = os.MkdirAll(filepath.Dir(dest), 0777)
	if err != nil {
		return nil, fmt.Errorf("download: dest mkdir error, path=%s, %w", dest, err)
	}

	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, fmt.Errorf("download: open error, path=%s, %w", dest, err)
	}
	defer func() {
		err := destFile.Close()
		if err != nil {
			log.Fatal(fmt.Errorf("download: close error, path=%s, %w", dest, err))
		}
	}()

	_, err = io.Copy(destFile, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("download: write error, path=%s, %w", dest, err)
	}

	if a.sha256 != "" {
		// return list of files
		return lszip(
			bytes.NewReader(body),
			int64(len(body)),
			modPath,
		)
	}
	return nil, nil
}

type extractMapPair struct {
	first  map[string]string
	second map[string]string
	item   *concurrency.Item[*assetMetadata, []string]
}

func buildExtractMap(i *concurrency.Item[*extractMapPair, map[string]string]) (map[string]string, error) {
	m := map[string]string{}
	paths := i.Data.item.Output
	for _, p := range paths {
		m[p] = path.Base(i.Data.item.Data.location)
	}
	return m, nil
}

func mergeExtractMapPair(i *concurrency.Item[*extractMapPair, map[string]string]) (map[string]string, error) {
	if i.Data.first == nil || i.Data.second == nil {
		return i.Output, nil
	}
	for p := range i.Data.second {
		i.Data.first[p] = i.Data.second[p]
	}
	return i.Data.first, nil
}

func (client *Client) extract(i *concurrency.Item[*assetMetadata, []string]) ([]string, error) {
	a := i.Data
	src := filepath.Join(client.tmpDir, path.Base(a.location))
	srcFile, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("extract: open error, path=%s, %w", src, err)
	}
	defer srcFile.Close()

	zdata, err := io.ReadAll(srcFile)
	if err != nil {
		return nil, fmt.Errorf("extract: read error, path=%s, %w", src, err)
	}

	err = unzip(
		bytes.NewReader(zdata),
		int64(len(zdata)),
		client.config.Workdir,
		modPath,
		func(p string) bool {
			if loc, ok := client.extractMap[p]; ok {
				return loc == path.Base(i.Data.location)
			}
			return true
		},
	)
	return nil, err
}

func (client *Client) downloadAssets(assets []*assetMetadata) ([]*concurrency.Item[*assetMetadata, []string], error) {
	var items []*concurrency.Item[*assetMetadata, []string]
	for _, a := range assets {
		items = append(items, &concurrency.Item[*assetMetadata, []string]{
			Data:   a,
			Output: nil,
			Err:    nil,
		})
	}

	con := client.config.Concurrency
	if len(items) < con {
		con = len(items)
	}

	err := concurrency.Execute(client.download, items, con)
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (client *Client) downloadAndExtract(assets []*assetMetadata) error {
	items, err := client.downloadAssets(assets)
	if err != nil {
		return err
	}

	// build extraction map to avoid overwriting newer files
	// convert []path into map[path]location
	var extMaps []*concurrency.Item[*extractMapPair, map[string]string]
	for _, i := range items {
		extMaps = append(extMaps, &concurrency.Item[*extractMapPair, map[string]string]{Data: &extractMapPair{item: i}})
	}
	err = concurrency.Execute(buildExtractMap, extMaps, client.config.Concurrency)
	if err != nil {
		return err
	}

	for len(extMaps) > 1 {
		var newMaps []*concurrency.Item[*extractMapPair, map[string]string]

		// group maps into pairs
		for i := 0; i < len(extMaps); i += 2 {
			if i+1 == len(extMaps) {
				newMaps = append(newMaps, extMaps[i])
				continue
			}
			newMaps = append(newMaps, &concurrency.Item[*extractMapPair, map[string]string]{
				Data: &extractMapPair{first: extMaps[i].Output, second: extMaps[i+1].Output},
			})
		}
		extMaps = newMaps

		err := concurrency.Execute(mergeExtractMapPair, extMaps, client.config.Concurrency)
		if err != nil {
			return err
		}
	}

	client.extractMap = extMaps[0].Output
	con := client.config.Concurrency
	if len(items) < con {
		con = len(items)
	}

	return concurrency.Execute(client.extract, items, con)
}

//go:generate msgp
type ComicListRequestBody struct {
	PageIndex int `msg:"page_index"`
	Kind      int `msg:"kind"` // 0 = char comics, 1 = guide comics
	ViewerID  int `msg:"viewer_id"`
}

func (client *Client) buildComicListRequest(comicListReq *ComicListRequestBody) (*retryablehttp.Request, error) {
	var body bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &body)
	err := msgp.Encode(encoder, comicListReq)
	if err != nil {
		return nil, err
	}
	err = encoder.Close()
	if err != nil {
		return nil, err
	}

	endpoint := client.config.CustomAPI
	if endpoint == "" {
		getAPIEndpoint(client.config.Region, apiComicEndpoint)
	}

	req, err := retryablehttp.NewRequest("POST", endpoint, &body)
	if err != nil {
		return nil, err
	}

	req.Header = client.header.Clone()
	req.Header.Set("APP_VER", "999.999.999")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("DEVICE", "2")
	req.Header.Set("GAME-APP-ID", fmt.Sprintf("%d", getViewerID(client.config.Region)))
	req.Header.Set("Referer", "app:/worldflipper_android_release.swf")
	req.Header.Set("UDID", "BC51B46F-B7D5-49C3-A651-62D255A49C8471D9")
	req.Header.Set("x-flash-version", "50,2,2,6")

	param, err := sha1Digest(req.Header.Get("UDID")+req.Header.Get("GAME-APP-ID")+apiComicEndpoint+body.String(), "")
	if err != nil {
		return nil, err
	}
	req.Header.Set("PARAM", param)

	return req, nil
}

type ComicMetadata struct {
	Episode      int    `json:"episode"`
	Title        string `json:"title"`
	CommenceTime string `json:"commenceTime"`
}

func (client *Client) parseComicList(data []byte) (int, []*assetMetadata, []*ComicMetadata, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	jsonParsed, err := gabs.ParseJSONDecoder(dec)
	if err != nil || !jsonParsed.ExistsP("data.comic_list") {
		return 0, nil, nil, err
	}

	count, err := jsonParsed.Path("data.total_count").Data().(json.Number).Int64()
	if err != nil {
		return 0, nil, nil, fmt.Errorf("parseComicList: unable to parse total count, %w", err)
	}

	var assets []*assetMetadata
	var comics []*ComicMetadata
	for _, child := range jsonParsed.Path("data.comic_list").Children() {
		episode, _ := child.Path("episode").Data().(json.Number).Int64()
		dest := filepath.Join(client.config.Workdir, fmt.Sprintf("%d", episode))
		assets = append(assets, &assetMetadata{
			location: child.Path("media_image.main").Data().(string),
			dest:     dest,
			sha256:   "",
		})
		assets = append(assets, &assetMetadata{
			location: child.Path("media_image.thumbnail_s").Data().(string),
			dest:     dest,
			sha256:   "",
		})
		assets = append(assets, &assetMetadata{
			location: child.Path("media_image.thumbnail_l").Data().(string),
			dest:     dest,
			sha256:   "",
		})
		comics = append(comics, &ComicMetadata{
			Episode:      int(episode),
			Title:        child.Path("title").Data().(string),
			CommenceTime: child.Path("commence_time").Data().(string),
		})
	}

	return int(count), assets, comics, nil
}

type comicListOutput struct {
	assets []*assetMetadata
	comics []*ComicMetadata
}

func (client *Client) downloadComicList(i *concurrency.Item[*ComicListRequestBody, *comicListOutput]) (*comicListOutput, error) {
	comicListReq, err := client.buildComicListRequest(i.Data)
	if err != nil {
		return nil, err
	}

	comicList, err := client.fetchMsgp(comicListReq)
	if err != nil {
		return nil, err
	}

	_, assets, comics, err := client.parseComicList(comicList)
	if err != nil {
		return nil, err
	}

	return &comicListOutput{assets: assets, comics: comics}, nil
}

func (client *Client) fetchComicsMetadata(kind int, version string) ([]*assetMetadata, error) {
	client.header.Set("RES_VER", version)

	comicListReq, err := client.buildComicListRequest(&ComicListRequestBody{
		ViewerID:  getViewerID(client.config.Region),
		Kind:      kind,
		PageIndex: 0,
	})
	if err != nil {
		return nil, err
	}

	comicList, err := client.fetchMsgp(comicListReq)
	if err != nil {
		return nil, err
	}

	count, assets, comics, err := client.parseComicList(comicList)
	if err != nil {
		return nil, err
	}

	var items []*concurrency.Item[*ComicListRequestBody, *comicListOutput]
	pages := int(math.Ceil(float64(count) / float64(len(comics))))
	for i := 1; i <= pages; i++ {
		items = append(items, &concurrency.Item[*ComicListRequestBody, *comicListOutput]{
			Data: &ComicListRequestBody{
				ViewerID:  getViewerID(client.config.Region),
				Kind:      kind,
				PageIndex: i,
			},
			Output: nil,
			Err:    nil,
		})
	}

	con := client.config.Concurrency
	if len(items) < con {
		con = len(items)
	}

	err = concurrency.Execute(client.downloadComicList, items, con)
	if err != nil {
		return nil, err
	}

	for _, i := range items {
		if i.Output != nil {
			if i.Output.comics != nil {
				comics = append(comics, i.Output.comics...)
			}
			if i.Output.assets != nil {
				assets = append(assets, i.Output.assets...)
			}
		}
	}
	sort.Slice(comics, func(i, j int) bool {
		return comics[i].Episode < comics[j].Episode
	})

	f, err := os.OpenFile(filepath.Join(client.config.Workdir, "metadata.json"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	err = enc.Encode(comics)
	if err != nil {
		return nil, err
	}

	return assets, nil
}

// FetchAssetsFromAPI fetches metadata from API then download and extract the assets archives.
func (client *Client) FetchAssetsFromAPI(fetchComics int) error {
	if fetchComics < 0 || fetchComics > 2 {
		log.Println("[WARN] Invalid comics id supplied, fetching character comics (1) instead")
		fetchComics = 1
	}
	if client.config.Region == RegionCN {
		log.Println("[WARN] CN region is untested due to region block")
	}

	endpoint := client.config.CustomAPI
	if endpoint == "" {
		endpoint = getAPIEndpoint(client.config.Region, apiAssetEndpoint)
	}

	log.Println("[INFO] Fetching asset metadata, clientVersion=" + client.config.Version)
	metadataReq, err := retryablehttp.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}

	metadataReq.Header = *client.header
	metadata, err := client.fetchMsgp(metadataReq)
	if err != nil {
		return err
	}

	err = os.MkdirAll(client.config.Workdir, 0777)
	if err != nil {
		return err
	}

	if fetchComics != 1 && fetchComics != 2 {
		tmpDir, err := os.MkdirTemp(client.config.Workdir, "fetchtmp")
		if err != nil {
			return err
		}
		defer func() {
			err := os.RemoveAll(tmpDir)
			if err != nil {
				log.Fatal(fmt.Errorf("FetchAssetsFromAPI: remove error, path=%s, %w", tmpDir, err))
			}
		}()

		client.tmpDir = tmpDir
	}

	latestVersion, assets, err := client.parseMetadata(metadata, fetchComics != 1 && fetchComics != 2)
	if err != nil {
		return err
	}

	if fetchComics != 1 && fetchComics != 2 {
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

	log.Printf("[INFO] Fetching comics list, type=%d, latestVersion=%s\n", fetchComics, latestVersion)
	assets, err = client.fetchComicsMetadata(fetchComics-1, latestVersion)
	if err != nil {
		return err
	}

	log.Printf("[INFO] Downloading comics, fileCount=%d\n", len(assets))
	_, err = client.downloadAssets(assets)
	return err
}
