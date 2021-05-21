package config

import (
	"io/ioutil"
	"path"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

func newTestConnectionFactory() (db.ConnectionFactory, error) {
	connFac, err := db.NewConnectionFactory(path.Join("test", "reconciler-test.yaml"), "configManagement")
	if err != nil {
		return connFac, err
	}

	if _, ok := connFac.(*db.SqliteConnectionFactory); ok {
		//get connection
		conn, err := connFac.NewConnection()
		if err != nil {
			panic(err)
		}

		//read DDL (test-table structure)
		ddl, err := ioutil.ReadFile(path.Join("test", "configuration-management.sql"))
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
