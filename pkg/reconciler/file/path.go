package file

import (
	"github.com/pkg/errors"
	"os"
)

const (
	temporaryFilePattern = "temp-file-*.yaml"
)

// CleanupFunc defines the contract for removing a temporary kubeconfig file.
type CleanupFunc func() error

// CreateTempFileWith returns a filesystem path to a generated temporary file with the given content.
// In order to ensure proper cleanup you should always call the returned CleanupFunc using `defer` statement.
func CreateTempFileWith(content string) (resPath string, cf CleanupFunc, err error) {
	resPath, err = createTemporaryFile(content)
	if err != nil {
		return "", nil, err
	}

	cf = func() error {
		if _, err := os.Stat(resPath); err == nil {
			return os.Remove(resPath)
		}
		return nil
	}

	return
}

func createTemporaryFile(content string) (string, error) {
	tmpFile, err := os.CreateTemp(os.TempDir(), temporaryFilePattern)
	if err != nil {
		return "", errors.Wrap(err, "Failed to generate a temporary file")
	}

	resPath := tmpFile.Name()
	if _, err = tmpFile.Write([]byte(content)); err != nil {
		return "", errors.Wrapf(err, "Failed to write to the temporary file: %s", resPath)
	}

	if err := tmpFile.Close(); err != nil {
		return "", errors.Wrapf(err, "Failed to close the temporary file: %s", resPath)
	}

	return resPath, nil
}
