package db

import (
	"io/ioutil"
	"path"
)

func NewTestConnectionFactory() (ConnectionFactory, error) {
	configDir := path.Join("..", "..", "configs")
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
