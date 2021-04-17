package db

type Connection interface {
	Query(sql string) interface{}
	Insert(sql string) (int, error)
	Update(sql string) (int, error)
	Delete(sql string) (int, error)
	Close() error
}

type ConnectionFactory interface {
	NewConnection() (Connection, error)
}
