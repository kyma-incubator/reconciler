package db

import (
	"fmt"
	"io/ioutil"
	"path"

	file "github.com/kyma-incubator/reconciler/pkg/files"
)

func NewTestConnectionFactory() (ConnectionFactory, error) {
	configDir, err := resolveConfigsDir()
	if err != nil {
		return nil, err
	}
	connFac, err := NewConnectionFactory(path.Join(configDir, "reconciler-unittest.yaml"))
	if err != nil {
		return connFac, err
	}

	if _, ok := connFac.(*SqliteConnectionFactory); ok {
		//get connection
		conn, err := connFac.NewConnection()
		if err != nil {
			panic(err)
		}

		//read DDL (test-table structure)
		ddl, err := ioutil.ReadFile(path.Join(configDir, "db", "sqlite", "reconciler.sql"))
		if err != nil {
			panic(err)
		}

		//populate DB schema
		_, err = conn.Exec(string(ddl))
		if err != nil {
			panic(err)
		}
	}

	return connFac, nil

}

func resolveConfigsDir() (string, error) {
	configsDir := path.Join("..", "..", "configs")
	for i := 0; i < 2; i++ {
		if file.DirExists(configsDir) {
			return configsDir, nil
		}
		configsDir = path.Join("..", configsDir)
	}
	return "", fmt.Errorf("Failed to resolve 'configs' directory")
}
