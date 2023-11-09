package assets

import _ "embed"

//go:embed pathlist
var PathList string

//go:embed item_white.png
var ItemWhite []byte

//go:embed item_bronze.png
var ItemBronze []byte

//go:embed item_silver.png
var ItemSilver []byte

//go:embed item_gold.png
var ItemGold []byte

//go:embed item_rainbow.png
var ItemRainbow []byte
