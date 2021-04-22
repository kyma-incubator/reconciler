package db

import (
	"bytes"
	"fmt"
)

type Query struct {
	conn          Connection
	entity        DatabaseEntity
	columnHandler *ColumnHandler
	buffer        bytes.Buffer
}

type Select struct {
	*Query
	args []interface{}
}

func (s *Select) Where(sqlWhere string, args ...interface{}) *Select {
	s.buffer.WriteString(fmt.Sprintf(" WHERE %s", sqlWhere))
	s.args = append(s.args, args...)
	return s
}

func (s *Select) Group(sqlGroup string) *Select {
	s.buffer.WriteString(fmt.Sprintf(" GROUP BY %s", sqlGroup))
	return s
}

func (s *Select) Order(sqlOrder string) *Select {
	s.buffer.WriteString(fmt.Sprintf(" ORDER BY %s", sqlOrder))
	return s
}

func (s *Select) GetOne() (DatabaseEntity, error) {
	row := s.conn.QueryRow(s.buffer.String(), s.args...)
	return s.entity, s.columnHandler.Synchronize(row, s.entity)
}

func (s *Select) GetMany() ([]DatabaseEntity, error) {
	rows, err := s.conn.Query(s.buffer.String(), s.args...)
	if err != nil {
		return nil, err
	}
	result := []DatabaseEntity{}
	for rows.Next() {
		entity := s.entity.New()
		stc, err := NewColumnHandler(entity)
		if err != nil {
			return result, err
		}
		if err := stc.Synchronize(rows, entity); err != nil {
			return result, err
		}
		result = append(result, entity)
	}
	return result, nil
}

type Insert struct {
	*Query
}

func (i *Insert) Exec() error {
	if err := i.columnHandler.Validate(); err != nil {
		return err
	}
	row := i.conn.QueryRow(i.buffer.String(), i.columnHandler.ColumnValues(true)...)
	return i.columnHandler.Synchronize(row, i.entity)
}

func NewQuery(conn Connection, entity DatabaseEntity) (*Query, error) {
	columnHandler, err := NewColumnHandler(entity)
	if err != nil {
		return nil, err
	}
	return &Query{
		conn:          conn,
		entity:        entity,
		columnHandler: columnHandler,
	}, nil
}

func (q *Query) Select() *Select {
	q.buffer.WriteString(fmt.Sprintf("SELECT %s FROM %s", q.columnHandler.ColumnNamesCsv(false), q.entity.Table()))
	return &Select{q, []interface{}{}}
}

func (q *Query) Insert() *Insert {
	q.buffer.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		q.entity.Table(), q.columnHandler.ColumnNamesCsv(true), q.columnHandler.ColumnValuesPlaceholderCsv(true), q.columnHandler.ColumnNamesCsv(false)))
	return &Insert{q}
}
