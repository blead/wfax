package wf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/blead/wfax/assets"
	"github.com/blead/wfax/pkg/concurrency"
	"github.com/disintegration/imaging"
	"github.com/hashicorp/go-multierror"
)

const ENHANCED_99_RARITY = 6

// SpriterConfig is the configuration for the spriter.
type SpriterConfig struct {
	SrcPath     string
	DestPath    string
	SpritePath  string
	Scale       float32
	Concurrency int
	Eliyabot    bool
}

// DefaultSpriterConfig generates a default configuration.
func DefaultSpriterConfig() *SpriterConfig {
	return &SpriterConfig{
		SrcPath:     "",
		DestPath:    "",
		SpritePath:  "item/sprite_sheet",
		Scale:       4,
		Concurrency: 5,
		Eliyabot:    false,
	}
}

// Spriter extracts and crops WF sprites.
type Spriter struct {
	config      *SpriterConfig
	backgrounds map[int]image.Image
}

// NewSpriter creates a new spriter with the supplied configuration.
// If the configuration is nil, use DefaultSpriterConfig.
func NewSpriter(config *SpriterConfig) (*Spriter, error) {
	def := DefaultSpriterConfig()
	if def == nil {
		return nil, fmt.Errorf("NewSpriter: default configuration is nil")
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
	if config.SpritePath == "" {
		config.SpritePath = "item/sprite_sheet"
	}

	if config.Scale == 0 {
		config.Scale = 4
	}

	if config.Concurrency == 0 {
		config.Concurrency = 5
	}

	backgrounds := make(map[int]image.Image)

	if config.Eliyabot {
		var err error
		backgrounds[1], err = imaging.Decode(bytes.NewReader(assets.ItemWhite))
		if err != nil {
			return nil, err
		}
		backgrounds[2], err = imaging.Decode(bytes.NewReader(assets.ItemBronze))
		if err != nil {
			return nil, err
		}
		backgrounds[3], err = imaging.Decode(bytes.NewReader(assets.ItemSilver))
		if err != nil {
			return nil, err
		}
		backgrounds[4], err = imaging.Decode(bytes.NewReader(assets.ItemGold))
		if err != nil {
			return nil, err
		}
		backgrounds[5], err = imaging.Decode(bytes.NewReader(assets.ItemRainbow))
		if err != nil {
			return nil, err
		}
		backgrounds[ENHANCED_99_RARITY], err = imaging.Decode(bytes.NewReader(assets.ItemRainbowEnhanced))
		if err != nil {
			return nil, err
		}
		for i, img := range backgrounds {
			size := int(24 * config.Scale)
			backgrounds[i] = imaging.Resize(img, size, size, imaging.NearestNeighbor)
		}
	}

	return &Spriter{config: config, backgrounds: backgrounds}, nil
}

type spriteParams struct {
	name    string
	devname string
	width   int
	height  int
	x       int
	y       int
	rotate  bool
}

func parseAtlas(atlas []byte) ([]*spriteParams, error) {
	dec := json.NewDecoder(bytes.NewReader(atlas))
	dec.UseNumber()
	jsonParsed, err := gabs.ParseJSONDecoder(dec)
	if err != nil {
		return nil, err
	}

	sprites := make(map[string]*spriteParams)
	for idx, sprite := range jsonParsed.Children() {
		var params spriteParams

		x, err := sprite.Search("x").Data().(json.Number).Int64()
		if err != nil {
			return nil, fmt.Errorf("parseAtlas: unable to parse x, idx=%d, %w", idx, err)
		}
		params.x = int(x)
		y, err := sprite.Search("y").Data().(json.Number).Int64()
		if err != nil {
			return nil, fmt.Errorf("parseAtlas: unable to parse y, idx=%d, %w", idx, err)
		}
		params.y = int(y)

		key := fmt.Sprintf("%d|%d", params.x, params.y)

		// ignore duplicates
		if sprites[key] == nil {
			var ok bool
			params.name, ok = sprite.Search("n").Data().(string)
			if !ok {
				return nil, fmt.Errorf("parseAtlas: unable to parse name, idx=%d", idx)
			}
			params.devname = path.Base(params.name)
			width, err := sprite.Search("w").Data().(json.Number).Int64()
			if err != nil {
				return nil, fmt.Errorf("parseAtlas: unable to parse width, idx=%d, devname=%s, %w", idx, params.name, err)
			}
			params.width = int(width)
			height, err := sprite.Search("h").Data().(json.Number).Int64()
			if err != nil {
				return nil, fmt.Errorf("parseAtlas: unable to parse height, idx=%d, devname=%s, %w", idx, params.name, err)
			}
			params.height = int(height)

			if sprite.Exists("r") && sprite.Search("r").String() == "true" {
				params.rotate = true
			}

			sprites[key] = &params
		}
	}

	var output []*spriteParams
	for _, params := range sprites {
		output = append(output, params)
	}

	return output, nil
}

func parseEquipmentMaps(equipmentsJSON []byte, equipmentEnhancementsJSON []byte) (rarityMap map[string]int, enhanced99Map map[string]bool, err error) {
	equipmentsParsed, err := gabs.ParseJSON(equipmentsJSON)
	if err != nil {
		return nil, nil, err
	}
	equipmentEnhancementsParsed, err := gabs.ParseJSON(equipmentEnhancementsJSON)
	if err != nil {
		return nil, nil, err
	}

	rarityMap = make(map[string]int)
	enhanced99Map = make(map[string]bool)
	for id, equipment := range equipmentsParsed.ChildrenMap() {
		devname, ok := equipment.Path("0.0").Data().(string)
		if !ok {
			return nil, nil, fmt.Errorf("parseEquipmentRarityMap: unable to parse devname, id=%s", id)
		}
		rarityStr, ok := equipment.Path("0.11").Data().(string)
		if !ok {
			return nil, nil, fmt.Errorf("parseEquipmentRarityMap: unable to parse rarity, id=%s, devname=%s", id, devname)
		}
		rarity, err := strconv.ParseInt(rarityStr, 10, 32)
		if err != nil {
			return nil, nil, err
		}
		rarityMap[devname] = int(rarity)

		if equipmentEnhancementsParsed.Exists(id) {
			enhanced99Map[devname] = true
		}
	}

	return rarityMap, enhanced99Map, nil
}

func (spriter *Spriter) extractAssets() (spriteSheet []byte, spriteAtlas []byte, equipments []byte, equipmentEnhancements []byte, err error) {

	targets := []*concurrency.Item[*extractParams, []byte]{
		{Data: &extractParams{path: spriter.config.SpritePath, parsers: []parser{&pngParser{}}}},
		{Data: &extractParams{path: spriter.config.SpritePath, parsers: []parser{&amf3Parser{ext: ".atlas"}}}},
	}

	if spriter.config.Eliyabot {
		targets = append(
			targets,
			&concurrency.Item[*extractParams, []byte]{
				Data: &extractParams{path: "item/equipment", parsers: []parser{&orderedmapParser{}}},
			},
			&concurrency.Item[*extractParams, []byte]{
				Data: &extractParams{path: "equipment_enhancement/equipment_enhancement", parsers: []parser{&orderedmapParser{}}},
			},
		)
	}

	err = concurrency.Dispatcher(
		func(i *concurrency.Item[*extractParams, []byte]) ([]*concurrency.Item[*extractParams, []byte], error) {
			var output []*concurrency.Item[*extractParams, []byte]
			if i.Output == nil {
				output = append(output, i)
			}
			return output, nil
		},
		func(i *concurrency.Item[*extractParams, []byte]) ([]byte, error) {
			parser := i.Data.parsers[0]
			src, err := parser.getSrc(i.Data.path, &ExtractorConfig{SrcPath: spriter.config.SrcPath})
			if err != nil {
				return nil, err
			}

			srcFile, err := os.Open(src)
			if err != nil {
				return nil, fmt.Errorf("extractAssets: src open error, src=%s, %w", src, err)
			}
			defer srcFile.Close()

			data, err := io.ReadAll(srcFile)
			if err != nil {
				return nil, fmt.Errorf("extractAssets: src read error, src=%s, %w", src, err)
			}
			data, err = parser.parse(data, &ExtractorConfig{})
			if err != nil {
				return nil, fmt.Errorf("extractAssets: src parse error, src=%s, %w", src, err)
			}

			return data, nil
		},
		targets,
		spriter.config.Concurrency,
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if spriter.config.Eliyabot {
		return targets[0].Output, targets[1].Output, targets[2].Output, targets[3].Output, nil
	}
	return targets[0].Output, targets[1].Output, nil, nil, nil
}

func (spriter *Spriter) processSprite(sheet image.Image, params *spriteParams, rarity int) error {
	// skip non-equipment
	if spriter.config.Eliyabot && rarity == 0 {
		return nil
	}

	destDir := spriter.config.SpritePath
	if spriter.config.Eliyabot {
		destDir = "eliyabot/item/equipment"
	}

	abisoul := false
	filename := params.devname
	if strings.Contains(params.name, "ability_soul") {
		abisoul = true
		filename = filename + "_soul"
	} else if rarity == ENHANCED_99_RARITY {
		filename = filename + "_enhanced99"
	} else if strings.HasSuffix(params.name, "_lv70") {
		filename = filename + "_enhanced"
	}

	dest := addExt(filepath.Join(
		spriter.config.DestPath,
		outputAssetsDir,
		filepath.FromSlash(destDir),
		filename,
	), ".png")

	os.MkdirAll(filepath.Dir(dest), 0777)
	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("processSprite: dest open error, dest=%s, %w", dest, err)
	}
	defer func() {
		err := destFile.Close()
		if err != nil {
			log.Fatal(fmt.Errorf("processSprite: dest close error, dest=%s, %w", dest, err))
		}
	}()

	img := imaging.Crop(sheet, image.Rect(params.x, params.y, params.x+params.width, params.y+params.height))
	if params.rotate {
		params.width, params.height = params.height, params.width
		img = imaging.Rotate90(img)
	}

	scaledWidth := int(float32(params.width) * spriter.config.Scale)
	scaledHeight := int(float32(params.height) * spriter.config.Scale)
	img = imaging.Resize(img, scaledWidth, scaledHeight, imaging.NearestNeighbor)

	if spriter.config.Eliyabot {
		var bg image.Image
		if abisoul {
			size := int(24 * spriter.config.Scale)
			bg = imaging.New(size, size, color.Transparent)
		} else {
			bg = spriter.backgrounds[rarity]
		}
		img = imaging.OverlayCenter(bg, img, 1)
	}

	return imaging.Encode(destFile, img, imaging.PNG, imaging.PNGCompressionLevel(png.BestCompression))
}

func (spriter *Spriter) processAssets(sheet image.Image, atlas []*spriteParams, rarity map[string]int, enhanced99 map[string]bool) error {

	var items []*concurrency.Item[*spriteParams, bool]
	for _, params := range atlas {
		items = append(items, &concurrency.Item[*spriteParams, bool]{Data: params})
	}

	return concurrency.Dispatcher(
		func(i *concurrency.Item[*spriteParams, bool]) ([]*concurrency.Item[*spriteParams, bool], error) {
			var output []*concurrency.Item[*spriteParams, bool]
			if !i.Output {
				output = append(output, i)
			}
			return output, nil
		},
		func(i *concurrency.Item[*spriteParams, bool]) (bool, error) {
			if strings.HasSuffix(i.Data.devname, "_lv0") {
				i.Data.devname = i.Data.devname[:len(i.Data.devname)-4]
			} else if strings.HasSuffix(i.Data.devname, "_lv70") {
				i.Data.devname = i.Data.devname[:len(i.Data.devname)-5]
			}

			if enhanced99[i.Data.devname] && !strings.HasSuffix(i.Data.name, "_lv0") {
				return true, multierror.Append(
					spriter.processSprite(sheet, i.Data, rarity[i.Data.devname]),
					spriter.processSprite(sheet, i.Data, ENHANCED_99_RARITY),
				).ErrorOrNil()
			}
			return true, spriter.processSprite(sheet, i.Data, rarity[i.Data.devname])
		},
		items,
		spriter.config.Concurrency,
	)
}

// ExtractSprite extracts and processes sprite assets.
func (spriter *Spriter) ExtractSprite() error {
	log.Println("[INFO] Extracting sprites")

	sheet, atlasJSON, equipmentsJSON, equipmentEnhancementsJSON, err := spriter.extractAssets()
	if err != nil {
		return err
	}

	sheetImage, err := imaging.Decode(bytes.NewReader(sheet))
	if err != nil {
		return err
	}

	atlas, err := parseAtlas(atlasJSON)
	if err != nil {
		return err
	}

	var rarity map[string]int
	var enhanced99 map[string]bool
	if spriter.config.Eliyabot {
		rarity, enhanced99, err = parseEquipmentMaps(equipmentsJSON, equipmentEnhancementsJSON)
		if err != nil {
			return err
		}
	} else {
		rarity = make(map[string]int)
		enhanced99 = make(map[string]bool)
	}

	return spriter.processAssets(sheetImage, atlas, rarity, enhanced99)
}
