package db

import (
	file "github.com/kyma-incubator/reconciler/pkg/files"
	"path/filepath"
)

//MigrationConfig is currently just a migrationConfig directory but could be extended at will for further configuration
type MigrationConfig string

//NoOpMigrationConfig is a shortcut to not have any migrationConfig at all
var NoOpMigrationConfig MigrationConfig

//DefaultMigrationConfig is a shortcut to the default migrations defined in the config repository
var DefaultMigrationConfig = MigrationConfig(DefaultMigrations())

func DefaultMigrations() string {
	return filepath.Join(file.Root, "configs", "db", "postgres")
}
