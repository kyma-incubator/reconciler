package db

import (
	"database/sql"
	"gorm.io/gorm"
)

const (
	MockRowsAffected = int64(999)
	MockLastInsertID = int64(111)
	MockEncryptorKey = "e286d76de2378ce776389a4f6df2b112"
)

type MockDbEntity struct {
	Col1 string `db:"notNull"`
	Col2 bool   `db:"readOnly"`
	Col3 int    `db:"encrypt"`
}

type MockConnection struct {
	query  string
	args   []interface{}
	dbType Type
}

type MockDataRow struct {
}

func (dr *MockDataRow) Scan(_ ...interface{}) error {
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
	return MockLastInsertID, nil
}

func (r *MockResult) RowsAffected() (int64, error) {
	return MockRowsAffected, nil
}

func (c *MockConnection) ID() string {
	return "mockConnectionID"
}

func (c *MockConnection) DB() *sql.DB {
	return nil
}

func (c *MockConnection) Encryptor() *Encryptor {
	encryptor, err := NewEncryptor(MockEncryptorKey)
	if err != nil {
		panic(err)
	}
	return encryptor
}

func (c *MockConnection) Ping() error {
	return nil
}

func (c *MockConnection) QueryRow(query string, args ...interface{}) (DataRow, error) {
	c.query = query
	c.args = args
	return &MockDataRow{}, nil
}

func (c *MockConnection) Query(query string, args ...interface{}) (DataRows, error) {
	c.query = query
	c.args = args
	return &MockDataRows{}, nil
}

func (c *MockConnection) QueryRowGorm(gormDB *gorm.DB) (DataRow, error) {
	c.query = GetString(gormDB)
	c.args = GetVars(gormDB)
	return &MockDataRow{}, nil
}
func (c *MockConnection) QueryGorm(gormDB *gorm.DB) (DataRows, error) {
	c.query = GetString(gormDB)
	c.args = GetVars(gormDB)
	return &MockDataRows{}, nil
}

func (c *MockConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	c.query = query
	c.args = args
	return &MockResult{}, nil
}

func (c *MockConnection) Begin() (*TxConnection, error) {
	return &TxConnection{}, nil
}

func (c *MockConnection) Close() error {
	return nil
}

func (c *MockConnection) DBStats() *sql.DBStats {
	return nil
}

func (c *MockConnection) Type() Type {
	if c.dbType == "" {
		return Mock
	}
	return c.dbType
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

func (fake *MockDbEntity) Equal(_ DatabaseEntity) bool {
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
