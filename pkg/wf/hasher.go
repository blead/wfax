package wf

import "path/filepath"

// Hasher hashes
type Hasher struct {
}

// HashAssetPath returns hashed asset path.
func (*Hasher) HashAssetPath(path string) (string, error) {
	return sha1Digest(filepath.ToSlash(path), digestSalt)
}
