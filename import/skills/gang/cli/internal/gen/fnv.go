package gen

import "unicode/utf16"

// FNV is FNV-1a 32-bit over the UTF-16 code units of s. It equals the engine's
// charCodeAt loop, so a non-ASCII title hashes the same on both sides.
func FNV(s string) uint32 {
	h := uint32(0x811c9dc5)
	for _, u := range utf16.Encode([]rune(s)) {
		h ^= uint32(u)
		h *= 0x01000193
	}
	return h
}
