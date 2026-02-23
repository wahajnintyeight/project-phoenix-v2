package helper

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"

	"github.com/joho/godotenv"
)

// EncryptAPIKey encrypts an API key using AES-GCM
func EncryptAPIKey(plaintext string) (string, error) {
	godotenv.Load()
	key := []byte(os.Getenv("ENCRYPTION_KEY"))

	if len(key) == 0 {
		return "", errors.New("ENCRYPTION_KEY not set in environment")
	}

	// Ensure key is 32 bytes for AES-256
	if len(key) < 32 {
		// Pad key if too short
		paddedKey := make([]byte, 32)
		copy(paddedKey, key)
		key = paddedKey
	} else if len(key) > 32 {
		// Truncate if too long
		key = key[:32]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAPIKey decrypts an encrypted API key
func DecryptAPIKey(encrypted string) (string, error) {
	godotenv.Load()
	key := []byte(os.Getenv("ENCRYPTION_KEY"))

	if len(key) == 0 {
		return "", errors.New("ENCRYPTION_KEY not set in environment")
	}

	// Ensure key is 32 bytes for AES-256
	if len(key) < 32 {
		paddedKey := make([]byte, 32)
		copy(paddedKey, key)
		key = paddedKey
	} else if len(key) > 32 {
		key = key[:32]
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
