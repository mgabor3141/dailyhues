package cache

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashImage generates a unique hash for image data
// This allows us to identify identical images even if they have different metadata/IDs
func HashImage(imageData []byte) string {
	hash := sha256.Sum256(imageData)
	return hex.EncodeToString(hash[:])
}
