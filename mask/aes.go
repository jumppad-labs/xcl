package mask

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// AES256GCMName is the name the AES-256-GCM masker writes into what it masks.
const AES256GCMName = "aes-256-gcm"

// EncryptAES256GCM returns a reversible masker that encrypts each value with
// AES-256 in GCM mode under key, which must be 32 bytes. Every value is sealed
// with a fresh random nonce, so masking one value twice gives different
// output. The masked value is a JSON string holding the base64 encoding of
// the nonce followed by the ciphertext. It opens only under the same key.
//
// The key is copied, so changing the slice afterwards has no effect. Keep it
// wherever the application keeps its secrets: state encrypted under one key
// cannot be read without it.
func EncryptAES256GCM(key []byte) (Masker, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("an AES-256-GCM key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(append([]byte(nil), key...))
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &aesGCM{aead: aead}, nil
}

type aesGCM struct {
	aead cipher.AEAD
}

func (a *aesGCM) Name() string { return AES256GCMName }

func (a *aesGCM) Mask(value json.RawMessage) (json.RawMessage, error) {
	nonce := make([]byte, a.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("unable to read a nonce: %w", err)
	}

	sealed := a.aead.Seal(nonce, nonce, value, nil)

	return json.Marshal(base64.StdEncoding.EncodeToString(sealed))
}

func (a *aesGCM) Unmask(masked json.RawMessage) (json.RawMessage, error) {
	var encoded string
	if err := json.Unmarshal(masked, &encoded); err != nil {
		return nil, fmt.Errorf("the masked value is not a string: %w", err)
	}

	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("the masked value is not base64: %w", err)
	}

	nonceSize := a.aead.NonceSize()
	if len(sealed) < nonceSize {
		return nil, fmt.Errorf("the masked value is too short")
	}

	opened, err := a.aead.Open(nil, sealed[:nonceSize], sealed[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("unable to decrypt, the key may be wrong: %w", err)
	}

	return opened, nil
}
