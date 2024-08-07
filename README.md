# World Flipper Asset Extractor CLI

This project is implemented as a slightly more automation-friendly alternative to existing tools.
It takes many inspirations and references from https://github.com/ScripterSugar/wdfp-extractor.
Thank you for your hard work.

## Installation

### Binary Releases
See [Releases](https://github.com/blead/wfax/releases).

### go install
```sh
go install github.com/blead/wfax@latest
```

### Build from Source
```sh
git clone https://github.com/blead/wfax
cd wfax
go build
```

## Usage
Fetch new (`diff-only`) raw assets for version `1.600.0` into `./dump` directory:
```sh
wfax fetch --diff-only --version 1.600.0 ./dump
```

Fetch raw assets from custom API and CDN endpoints (file URIs are also supported):
```sh
wfax fetch --custom-api file:///assets/asset_lists/en-android-full.json --custom-cdn file:///.cdn ./dump
```

Fetch character comics (`--comics 1`) with `10` maximum concurrent requests into `./comics` directory:
```sh
wfax fetch --comics 1 --concurrency 10 ./comics
```

Extract assets with `2` spaces indentation from `./dump` into `./output`:
```sh
wfax extract --indent 2 ./dump ./output
```

Extract character image assets for eliyabot:
```sh
wfax extract --eliyabot --no-default-paths ./dump ./output
```

Extract equipment image assets for eliyabot:
```sh
wfax sprite --eliyabot ./dump ./output
```

Pack extracted files in `./output` into raw assets in `./repack`:
```sh
wfax pack ./output ./repack
```

For more detailed information, use `wfax help`.

## Supported Assets
The main focus currently is extracting text files so other assets are not fully supported.
* Ordered Maps
* Action/Enemy DSL files
* Image assets for EliyaBot
* Comics
