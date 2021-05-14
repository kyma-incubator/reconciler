package db

import (
	"database/sql"
)

const (
	MockRowsAffected = int64(999)
	MockLastInsertId = int64(111)
)

type MockDbEntity struct {
	Col1 string `db:"notNull"`
	Col2 bool   `db:"readOnly"`
	Col3 int
}

type MockConnection struct {
	query string
	args  []interface{}
}

type MockDataRow struct {
}

func (dr *MockDataRow) Scan(dest ...interface{}) error {
	return nil
}

type MockDataRows struct {
	*MockDataRow
}

func (dr *MockDataRows) Next() bool {
	return false
}

type MockResult struct {
}

func (r *MockResult) LastInsertId() (int64, error) {
	return MockLastInsertId, nil
}

func (r *MockResult) RowsAffected() (int64, error) {
	return MockRowsAffected, nil
}

func (c *MockConnection) QueryRow(query string, args ...interface{}) DataRow {
	c.query = query
	c.args = args
	return &MockDataRow{}
}
func (c *MockConnection) Query(query string, args ...interface{}) (DataRows, error) {
	c.query = query
	c.args = args
	return &MockDataRows{}, nil
}
func (c *MockConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	c.query = query
	c.args = args
	return &MockResult{}, nil
}
func (c *MockConnection) Begin() (*sql.Tx, error) {
	return nil, nil
}
func (c *MockConnection) Close() error {
	return nil
}

func (fake *MockDbEntity) String() string {
	return "I'm a mock entity"
}

func (fake *MockDbEntity) New() DatabaseEntity {
	return &MockDbEntity{}
}

func (fake *MockDbEntity) Table() string {
	return "mockTable"
}

func (fake *MockDbEntity) Equal(other DatabaseEntity) bool {
	return false
}

func (fake *MockDbEntity) Marshaller() *EntityMarshaller {
	syncer := NewEntityMarshaller(&fake)
	syncer.AddUnmarshaller("Col1", func(value interface{}) (interface{}, error) {
		return "col1", nil
	})
	syncer.AddUnmarshaller("Col2", func(value interface{}) (interface{}, error) {
		return true, nil
	})
	syncer.AddUnmarshaller("Col3", func(value interface{}) (interface{}, error) {
		return 3, nil
	})
	return syncer
}
