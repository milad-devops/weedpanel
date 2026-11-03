package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

type Vault struct{ aead cipher.AEAD }

func NewVault(encodedKey string) (*Vault, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, errors.New("APP_ENCRYPTION_KEY must be base64-encoded 32-byte key")
	}
	if len(key) != 32 {
		return nil, errors.New("APP_ENCRYPTION_KEY must decode to exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: aead}, nil
}

func (v *Vault) Encrypt(plain []byte) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return v.aead.Seal(nonce, nonce, plain, nil), nil
}

func (v *Vault) Decrypt(ciphertext []byte) ([]byte, error) {
	n := v.aead.NonceSize()
	if len(ciphertext) < n {
		return nil, errors.New("invalid ciphertext")
	}
	return v.aead.Open(nil, ciphertext[:n], ciphertext[n:], nil)
}
