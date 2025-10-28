package cache

import (
	"testing"
)

// TestHashImage_Deterministic tests that same data produces same hash
func TestHashImage_Deterministic(t *testing.T) {
	imageData := []byte("test image data")

	hash1 := HashImage(imageData)
	hash2 := HashImage(imageData)

	if hash1 != hash2 {
		t.Errorf("Expected same hash for same data, got %s and %s", hash1, hash2)
	}
}

// TestHashImage_Different tests that different data produces different hashes
func TestHashImage_Different(t *testing.T) {
	imageData1 := []byte("test image data 1")
	imageData2 := []byte("test image data 2")

	hash1 := HashImage(imageData1)
	hash2 := HashImage(imageData2)

	if hash1 == hash2 {
		t.Errorf("Expected different hashes for different data, both got %s", hash1)
	}
}

// TestHashImage_Format tests that hash is valid SHA256 hex string
func TestHashImage_Format(t *testing.T) {
	imageData := []byte("test image data")
	hash := HashImage(imageData)

	// SHA256 produces 32 bytes = 64 hex characters
	expectedLength := 64
	if len(hash) != expectedLength {
		t.Errorf("Expected hash length %d, got %d", expectedLength, len(hash))
	}

	// Check if hash contains only hex characters
	for _, char := range hash {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			t.Errorf("Hash contains non-hex character: %c", char)
		}
	}
}

// TestHashImage_Empty tests hashing empty data
func TestHashImage_Empty(t *testing.T) {
	imageData := []byte{}
	hash := HashImage(imageData)

	// Empty data should still produce a valid hash
	if len(hash) != 64 {
		t.Errorf("Expected hash length 64 even for empty data, got %d", len(hash))
	}

	// Known SHA256 hash of empty data
	expectedHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expectedHash {
		t.Errorf("Expected hash %s for empty data, got %s", expectedHash, hash)
	}
}

// TestHashImage_SameImageIdentified tests that identical images get same hash
func TestHashImage_SameImageIdentified(t *testing.T) {
	// Simulate two locales getting the same wallpaper image
	imageDataUS := []byte("wallpaper_image_binary_data_12345")
	imageDataJP := []byte("wallpaper_image_binary_data_12345") // Same image

	hashUS := HashImage(imageDataUS)
	hashJP := HashImage(imageDataJP)

	if hashUS != hashJP {
		t.Errorf("Expected same hash for identical images, got %s and %s", hashUS, hashJP)
	}
}

// TestHashImage_OneByteChange tests sensitivity to small changes
func TestHashImage_OneByteChange(t *testing.T) {
	imageData1 := []byte("test image data")
	imageData2 := []byte("test image datb") // Changed last character

	hash1 := HashImage(imageData1)
	hash2 := HashImage(imageData2)

	if hash1 == hash2 {
		t.Error("Expected different hashes for images differing by one byte")
	}
}

// BenchmarkHashImage benchmarks hashing performance
func BenchmarkHashImage(b *testing.B) {
	// Simulate a typical wallpaper image size (~300KB)
	imageData := make([]byte, 300*1024)
	for i := range imageData {
		imageData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HashImage(imageData)
	}
}

// BenchmarkHashImage_Small benchmarks hashing small images
func BenchmarkHashImage_Small(b *testing.B) {
	// Simulate a small thumbnail (~50KB)
	imageData := make([]byte, 50*1024)
	for i := range imageData {
		imageData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HashImage(imageData)
	}
}

// BenchmarkHashImage_Large benchmarks hashing large images
func BenchmarkHashImage_Large(b *testing.B) {
	// Simulate a large UHD image (~3MB)
	imageData := make([]byte, 3*1024*1024)
	for i := range imageData {
		imageData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HashImage(imageData)
	}
}
