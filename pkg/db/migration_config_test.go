package db

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrations(t *testing.T) {
	a := require.New(t)
	suffix := ".sql"
	a.NoError(filepath.Walk(DefaultMigrations(), func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if err != nil {
			return err
		}
		a.True(strings.HasSuffix(info.Name(), suffix), fmt.Sprintf("%s should have suffix %s", info.Name(), suffix))
		return nil
	}))
}
