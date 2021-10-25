package db

import (
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

const data = "I will be encrypted :-P"

func TestEncryptor(t *testing.T) {
	t.Run("Works not with empty key", func(t *testing.T) {
		_, err := NewEncryptor("")
		require.Error(t, err)
	})

	t.Run("Works not with non-HEX key", func(t *testing.T) {
		_, err := NewEncryptor("abc123!")
		require.Error(t, err)
	})

	t.Run("Is Decryptable", func(t *testing.T) {
		enc1 := newEncryptor(t)
		enc2 := newEncryptor(t)

		encData1, err := enc1.Encrypt("abc")
		require.NoError(t, err)
		require.True(t, enc1.Decryptable(encData1))
		require.False(t, enc2.Decryptable(encData1))

		encData2, err := enc2.Encrypt("xyz")
		require.NoError(t, err)
		require.False(t, enc1.Decryptable(encData2))
		require.True(t, enc2.Decryptable(encData2))
	})

	t.Run("Encrypt and decrypt", func(t *testing.T) {
		enc := newEncryptor(t)

		encData, err := enc.Encrypt(data)
		require.NoError(t, err)

		decData, err := enc.Decrypt(encData)
		require.NoError(t, err)

		require.Equal(t, data, decData)
	})

	t.Run("Verify idempotency", func(t *testing.T) {
		key, err := NewEncryptionKey()
		require.NoError(t, err)

		enc1, err := NewEncryptor(key)
		require.NoError(t, err)
		enc2, err := NewEncryptor(key)
		require.NoError(t, err)

		encData1, err := enc1.Encrypt(data)
		require.NoError(t, err)
		encData2, err := enc2.Encrypt(data)
		require.NoError(t, err)

		//decode with data1 with enc2 and data2 with enc1
		decData1, err := enc2.Decrypt(encData1)
		require.NoError(t, err)
		require.Equal(t, decData1, data)

		decData2, err := enc1.Decrypt(encData2)
		require.NoError(t, err)
		require.Equal(t, decData2, data)

		require.Equal(t, decData1, decData2)
	})

}

func TestReadKeyFile(t *testing.T) {
	t.Run("Read valid key", func(t *testing.T) {
		key, err := readKeyFile(filepath.Join("test", "valid.key"))
		require.NoError(t, err)
		require.NotEmpty(t, key)
	})
	t.Run("Read missing key", func(t *testing.T) {
		key, err := readKeyFile(filepath.Join("test", "donexist.key"))
		require.Error(t, err)
		require.Empty(t, key)
	})
	t.Run("Read wrong length key", func(t *testing.T) {
		key, err := readKeyFile(filepath.Join("test", "wronglength.key"))
		require.Error(t, err)
		require.Empty(t, key)
	})
	t.Run("Read invalid key", func(t *testing.T) {
		key, err := readKeyFile(filepath.Join("test", "invalid.key"))
		require.Error(t, err)
		require.Empty(t, key)
	})
}

func newEncryptor(t *testing.T) *Encryptor {
	key, err := NewEncryptionKey()
	require.NoError(t, err)

	enc, err := NewEncryptor(key)
	require.NoError(t, err)

	return enc
}
