package mask

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// HMACSHA256Name is the name the HMAC-SHA256 masker writes into what it masks.
const HMACSHA256Name = "hmac-sha256"

// HashHMACSHA256 returns a one-way masker that writes the hex encoded
// HMAC-SHA256 of each value under key, which must not be empty. The same value
// under the same key always gives the same output, so masked values can be
// correlated without being revealed, but they can never be recovered.
//
// The key is copied, so changing the slice afterwards has no effect.
func HashHMACSHA256(key []byte) (Masker, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("an HMAC-SHA256 key must not be empty")
	}

	return &hmacSHA256{key: append([]byte(nil), key...)}, nil
}

type hmacSHA256 struct {
	key []byte
}

func (h *hmacSHA256) Name() string { return HMACSHA256Name }

func (h *hmacSHA256) Mask(value json.RawMessage) (json.RawMessage, error) {
	digest := hmac.New(sha256.New, h.key)
	digest.Write(value)

	return json.Marshal(hex.EncodeToString(digest.Sum(nil)))
}
