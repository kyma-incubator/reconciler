package db

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestUnittestEncryptionKeyFile(t *testing.T) {
	a := require.New(t)
	fi, err := os.Stat(UnittestEncryptionKeyFile())
	a.NoError(err)
	a.Equal("unittest.key", fi.Name())
}
