package test

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func ReadFile(t *testing.T, file string) []byte {
	data, err := os.ReadFile(file)
	require.NoError(t, err)
	return data
}
