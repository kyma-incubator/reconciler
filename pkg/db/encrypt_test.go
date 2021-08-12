package db

import (
	"github.com/stretchr/testify/require"
	"testing"
)

const data = "I will be encrypted :-P"

func TestEncryptor(t *testing.T) {
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

}

func newEncryptor(t *testing.T) *Encryptor {
	key, err := NewKey()
	require.NoError(t, err)

	enc, err := NewEncryptor(key)
	require.NoError(t, err)

	return enc
}
