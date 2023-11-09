package encoding

import (
	"bytes"
	"image/color"
	"image/png"

	"github.com/disintegration/imaging"
)

func FitPNG(src []byte, width int, height int) ([]byte, error) {
	img, err := imaging.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}

	img = imaging.PasteCenter(
		imaging.New(width, height, color.Transparent),
		imaging.Fit(img, width, height, imaging.Lanczos),
	)

	var output bytes.Buffer
	err = imaging.Encode(&output, img, imaging.PNG, imaging.PNGCompressionLevel(png.BestCompression))
	if err != nil {
		return nil, err
	}

	return output.Bytes(), nil
}
