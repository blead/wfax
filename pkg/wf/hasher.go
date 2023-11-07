package wf

// Hasher hashes
type Hasher struct {
}

// HashAssetPath returns hashed asset path.
func (*Hasher) HashAssetPath(path string) (string, error) {
	return sha1Digest(path, digestSalt)
}
