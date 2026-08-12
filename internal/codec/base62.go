// Package codec provides base62 encoding for short code generation.
package codec

import (
	"crypto/rand"
	"math/big"
	"strings"
)

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Generate returns a cryptographically random base62 string of length n.
func Generate(n int) (string, error) {
	base := big.NewInt(int64(len(alphabet)))
	var sb strings.Builder
	sb.Grow(n)
	for range n {
		idx, err := rand.Int(rand.Reader, base)
		if err != nil {
			return "", err
		}
		sb.WriteByte(alphabet[idx.Int64()])
	}
	return sb.String(), nil
}

// IsValid reports whether s consists entirely of base62 characters.
func IsValid(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune(alphabet, c) {
			return false
		}
	}
	return true
}
