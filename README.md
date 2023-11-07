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
Fetch new (`diff-only`) assets for version `1.600.0` into `./dump` directory and extract them with `2` spaces indentation into `./output`:
```sh
wfax fetch --diff-only --version 1.600.0 ./dump && wfax extract --indent 2 ./dump ./output
```

For more information, use `wfax help`.

## Supported Assets
The main focus currently is extracting text files so other assets are not fully supported.
* Ordered Maps
* Action/Enemy DSL files
* Image assets for EliyaBot (requires a pathlist of internal names)
