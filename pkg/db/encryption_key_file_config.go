package db

import (
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"path/filepath"
)

// EncryptionKeyFileConfig is currently just an encryption key file but could be extended at will for further configuration
type EncryptionKeyFileConfig string

// UnittestEncryptionKeyFileConfig is a shortcut to the default unit test key
var UnittestEncryptionKeyFileConfig = EncryptionKeyFileConfig(UnittestEncryptionKeyFile())

func UnittestEncryptionKeyFile() string {
	return filepath.Join(file.Root, "configs", "encryption", "unittest.key")
}
