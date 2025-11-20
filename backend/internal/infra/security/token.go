package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

type RandomTokenGenerator struct {
	Size int
}

func (g RandomTokenGenerator) NewToken() (string, error) {
	size := g.Size
	if size <= 0 {
		size = 32
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("token: entropy read failed: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
