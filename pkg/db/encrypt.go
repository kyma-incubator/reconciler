package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" //nolint: gosec //using MD5 just for generating a checksum of the key
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"strings"
)

type Encryptor struct {
	keyID [16]byte
	aead  cipher.AEAD
}

func NewEncryptor(key string) (*Encryptor, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}

	return &Encryptor{
		aead:  aead,
		keyID: md5.Sum([]byte(key)), //nolint: gosec //using MD5 just for generating a checksum of the key
	}, nil
}

//NewKey generates a random 32 byte key for AES-256
func NewKey() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	return hex.EncodeToString(bytes), err
}

func newAEAD(key string) (cipher.AEAD, error) {
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return nil, errors.Wrap(err, "key is not a HEX string")
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

//KeyID returns the MD5 checksum of the current key as HEX string
func (e *Encryptor) KeyID() string {
	return fmt.Sprintf("%x", e.keyID)
}

func (e *Encryptor) Encrypt(data string) (string, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	enc := e.aead.Seal(nonce, nonce, []byte(data), nil)
	return fmt.Sprintf("%s%x", e.KeyID(), enc), nil //add keyID as prefix to the encrypted data
}

func (e *Encryptor) Decrypt(encData string) (string, error) {
	if !e.Decryptable(encData) {
		return "", fmt.Errorf("data cannot be decrypted because encryption key does not match")
	}

	enc, err := hex.DecodeString(strings.TrimPrefix(encData, e.KeyID())) //remove keyID from encrypted data
	if err != nil {
		return "", fmt.Errorf("failed to decode HEX string to bytes")
	}

	nonceSize := e.aead.NonceSize()
	nonce, cipherText := enc[:nonceSize], enc[nonceSize:]

	data, err := e.aead.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s", data), nil
}

//Decryptable verifies whether the encrypted data can be decrypted by this Encryptor instance
func (e *Encryptor) Decryptable(encData string) bool {
	return strings.HasPrefix(encData, e.KeyID()) //KeyID prefix of encrypted data has to match with current KeyID
}
