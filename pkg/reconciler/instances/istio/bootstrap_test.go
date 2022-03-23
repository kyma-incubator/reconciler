package istio

import (
	"go.uber.org/zap"
	"os"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestParsePaths(t *testing.T) {
	alwaysValidFn := func(path string, logger *zap.SugaredLogger) error { return nil }

	t.Run("parsePaths should parse a single path", func(t *testing.T) {
		//given
		paths := "/a/b/c"
		//when
		res, err := parsePaths(paths, alwaysValidFn, &zap.SugaredLogger{})
		//then
		require.NoError(t, err)
		require.Equal(t, 1, len(res))
		require.Equal(t, "/a/b/c", res[0])
	})
	t.Run("parsePaths should parse two paths", func(t *testing.T) {
		//given
		paths := "/a/b/c;/d/e/f"
		//when
		res, err := parsePaths(paths, alwaysValidFn, &zap.SugaredLogger{})
		//then
		require.NoError(t, err)
		require.Equal(t, 2, len(res))
		require.Equal(t, "/a/b/c", res[0])
		require.Equal(t, "/d/e/f", res[1])
	})
	t.Run("parsePaths should parse three paths", func(t *testing.T) {
		//given
		paths := "/a/b/c;/d/e/f;/g/h/i"
		//when
		res, err := parsePaths(paths, alwaysValidFn, &zap.SugaredLogger{})
		//then
		require.NoError(t, err)
		require.Equal(t, 3, len(res))
		require.Equal(t, "/a/b/c", res[0])
		require.Equal(t, "/d/e/f", res[1])
		require.Equal(t, "/g/h/i", res[2])
	})
	t.Run("parsePaths should return an error on an empty path", func(t *testing.T) {
		//given
		paths := ""
		//when
		_, err := parsePaths(paths, alwaysValidFn, &zap.SugaredLogger{})
		//then
		require.Error(t, err)
		require.Contains(t, err.Error(), "ISTIOCTL_PATH env variable is undefined or empty")
	})
	t.Run("parsePaths should return an error on an all-spaces path", func(t *testing.T) {
		//given
		paths := "   "
		//when
		_, err := parsePaths(paths, alwaysValidFn, &zap.SugaredLogger{})
		//then
		require.Error(t, err)
		require.Contains(t, err.Error(), "ISTIOCTL_PATH env variable is undefined or empty")
	})
	t.Run("parsePaths should return an error on paths containing just a colon", func(t *testing.T) {
		//given
		paths := ";"
		//when
		_, err := parsePaths(paths, alwaysValidFn, &zap.SugaredLogger{})
		//then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Invalid (empty) path provided")
	})
	t.Run("parsePaths should return an error on paths starting with an empty element", func(t *testing.T) {
		//given
		paths := ";/a/b/c"
		//when
		_, err := parsePaths(paths, alwaysValidFn, &zap.SugaredLogger{})
		//then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Invalid (empty) path provided")
	})
	t.Run("parsePaths should return an error on paths ending with an empty element", func(t *testing.T) {
		//given
		paths := "/a/b/c;"
		//when
		_, err := parsePaths(paths, alwaysValidFn, &zap.SugaredLogger{})
		//then
		require.Error(t, err)
		require.Contains(t, err.Error(), "Invalid (empty) path provided")
	})
	t.Run("parsePaths should return an error when validation function fails", func(t *testing.T) {
		//given
		paths := "/a/b/c;/d/e/f"
		isValidFn := func(s string, logger *zap.SugaredLogger) error {
			if s == "/d/e/f" {
				return errors.New("foo")
			}
			return nil
		}
		//when
		_, err := parsePaths(paths, isValidFn, &zap.SugaredLogger{})
		//then
		require.Error(t, err)
		require.Equal(t, "foo", err.Error())
	})
	t.Run("parsePaths should return an error when path is too long", func(t *testing.T) {
		//given
		pathPart9 := "/abcdefgh"
		paths := []string{}
		for i := 0; i < istioctlBinaryPathMaxLen/10+1; i++ {
			paths = append(paths, pathPart9)
		}

		//when
		_, err := parsePaths(strings.Join(paths, ";"), alwaysValidFn, &zap.SugaredLogger{})
		//then
		require.Error(t, err)
		require.Contains(t, err.Error(), "ISTIOCTL_PATH env variable exceeds the maximum istio path limit")
	})
}

func TestChmodExecutbale(t *testing.T) {
	t.Run("chmod to 0777", func(t *testing.T) {
		pathToFile := "tmp/dat1"
		t.Cleanup(func() {
			err := os.RemoveAll("tmp")
			require.NoError(t, err)
		})
		require.NoError(t, os.MkdirAll("tmp", 0777))
		d1 := []byte("hello\nworld\n")
		err := os.WriteFile(pathToFile, d1, 0000)
		require.NoError(t, err)
		err = chmodExecutbale(pathToFile, zap.NewNop().Sugar())
		require.NoError(t, err)
		stat, err := os.Stat(pathToFile)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0777), stat.Mode())
	})
	t.Run("chmodExecutable should return an error if file is not existing", func(t *testing.T) {
		pathToFile := "not-existing"
		err := chmodExecutbale(pathToFile, zap.NewNop().Sugar())
		require.Error(t, err)
	})
	t.Run("chmodExecutable should return an error if pathToFile is empty", func(t *testing.T) {
		pathToFile := ""
		err := chmodExecutbale(pathToFile, zap.NewNop().Sugar())
		require.Error(t, err)
	})
}

func TestValidatePath(t *testing.T) {
	t.Run("validatePath should return nil and change mode of file to 0777", func(t *testing.T) {
		pathToFile := "tmp/dat1"
		t.Cleanup(func() {
			err := os.RemoveAll("tmp")
			require.NoError(t, err)
		})
		require.NoError(t, os.MkdirAll("tmp", 0777))
		d1 := []byte("hello\nworld\n")
		err := os.WriteFile(pathToFile, d1, 0000)
		require.NoError(t, err)
		err = validatePath(pathToFile, zap.NewNop().Sugar())
		require.NoError(t, err)
		stat, err := os.Stat(pathToFile)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0777), stat.Mode())

	})
	t.Run("validatePath should return error if file is not existing", func(t *testing.T) {
		pathToFile := "not-existing"
		err := validatePath(pathToFile, zap.NewNop().Sugar())
		require.Error(t, err)
	})
	t.Run("validatePath should return error if path is empty", func(t *testing.T) {
		pathToFile := ""
		err := validatePath(pathToFile, zap.NewNop().Sugar())
		require.Error(t, err)
	})
}
