package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" //nolint: gosec //using MD5 just for generating a checksum of the key
	"crypto/rand"
	"encoding/hex"
	"fmt"
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"strings"
)

const keyIDLength = 15
const KeyLength = 32

type Encryptor struct {
	keyID [16]byte
	aead  cipher.AEAD
}

func NewEncryptor(key string) (*Encryptor, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("cannot create new encryptor instance because encryption key was an empty string")
	}

	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}

	return &Encryptor{
		aead:  aead,
		keyID: md5.Sum([]byte(key)), //nolint: gosec //using MD5 just for generating a checksum of the key
	}, nil
}

//NewEncryptionKey generates a random 32 byte key for AES-256
func NewEncryptionKey() (string, error) {
	bytes := make([]byte, KeyLength)
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

//KeyID returns the first characters of the MD5 keys checksum as HEX string
func (e *Encryptor) KeyID() string {
	return fmt.Sprintf("%x", e.keyID)[:keyIDLength]
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

func readKeyFile(encKeyFile string) (string, error) {
	if !file.Exists(encKeyFile) {
		return "", fmt.Errorf("encryption key file '%s' not found", encKeyFile)
	}
	encKeyBytes, err := ioutil.ReadFile(encKeyFile)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("failed to read encryption key file '%s'", encKeyFile))
	}
	length, err := hex.Decode(make([]byte, KeyLength), encKeyBytes)
	if err != nil {
		return "", errors.Wrap(err, "encryption key is not a valid HEX string")
	}
	if length != KeyLength {
		return "", fmt.Errorf("encryption key has to be %d bytes long (was %d)", KeyLength, length)
	}
	return string(encKeyBytes), nil
}
