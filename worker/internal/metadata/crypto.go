package metadata

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

const encryptedValueVersion byte = 1

type encryptor struct {
	key []byte
}

func newEncryptor(encodedKey string) (*encryptor, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, errors.New("APP_ENCRYPTION_KEY must be base64-encoded")
	}
	if len(key) != 32 {
		return nil, errors.New("APP_ENCRYPTION_KEY must decode to exactly 32 bytes")
	}
	return &encryptor{key: key}, nil
}

func (e *encryptor) encrypt(plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.New("could not generate encryption nonce")
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	value := make([]byte, 1+len(nonce)+len(ciphertext))
	value[0] = encryptedValueVersion
	copy(value[1:], nonce)
	copy(value[1+len(nonce):], ciphertext)
	return value, nil
}

func (e *encryptor) decrypt(value []byte) (string, error) {
	if len(value) == 0 || value[0] != encryptedValueVersion {
		return "", errors.New("unknown encrypted value format")
	}
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(value) < 1+gcm.NonceSize() {
		return "", errors.New("invalid encrypted value")
	}
	nonce := value[1 : 1+gcm.NonceSize()]
	plaintext, err := gcm.Open(nil, nonce, value[1+gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("could not decrypt stored secret")
	}
	return string(plaintext), nil
}
