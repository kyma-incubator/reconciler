package db

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

type Query struct {
	conn          Connection
	entity        DatabaseEntity
	columnHandler *ColumnHandler
	buffer        bytes.Buffer
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

func (q *Query) reset() {
	q.buffer = bytes.Buffer{}
}

func (q *Query) Select() *Select {
	q.buffer.WriteString(fmt.Sprintf("SELECT %s FROM %s", q.columnHandler.ColumnNamesCsv(false), q.entity.Table()))
	return &Select{q, []interface{}{}, nil}
}

func (q *Query) Insert() *Insert {
	q.buffer.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		q.entity.Table(), q.columnHandler.ColumnNamesCsv(true), q.columnHandler.ColumnValuesPlaceholderCsv(true), q.columnHandler.ColumnNamesCsv(false)))
	return &Insert{q}
}

func (q *Query) Delete() *Delete {
	q.buffer.WriteString(fmt.Sprintf("DELETE FROM %s", q.entity.Table()))
	return &Delete{q, []interface{}{}, nil}
}

type Select struct {
	*Query
	args []interface{}
	err  error
}

func (s *Select) Where(args map[string]interface{}) *Select {
	s.args, s.err = addWhereCondition(args, &s.buffer, s.columnHandler)
	return s
}

func (s *Select) GroupBy(args []string) *Select {
	if len(args) == 0 {
		return s
	}

	//get groupings
	grouping := []string{}
	for _, field := range args {
		col, err := s.columnHandler.ColumnName(field)
		if err != nil {
			s.err = err
			return s
		}
		grouping = append(grouping, fmt.Sprintf(" %s", col))
	}

	//render group condition
	s.buffer.WriteString(" GROUP BY")
	s.buffer.WriteString(strings.Join(grouping, ", "))
	return s
}

func (s *Select) OrderBy(args map[string]string) *Select {
	if len(args) == 0 {
		return s
	}

	//get sorted list of fields
	fields := make([]string, 0, len(args))
	for field := range args {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	//get orderings
	ordering := []string{}
	for _, field := range fields {
		col, err := s.columnHandler.ColumnName(field)
		if err != nil {
			s.err = err
			return s
		}
		ordering = append(ordering, fmt.Sprintf(" %s %s", col, args[field]))
	}

	//render order condition
	s.buffer.WriteString(" ORDER BY")
	s.buffer.WriteString(strings.Join(ordering, ", "))
	return s
}

func (s *Select) GetOne() (DatabaseEntity, error) {
	if s.err != nil {
		return nil, s.err
	}
	defer s.reset()
	row := s.conn.QueryRow(s.buffer.String(), s.args...)
	return s.entity, s.columnHandler.Synchronize(row, s.entity)
}

func (s *Select) GetMany() ([]DatabaseEntity, error) {
	if s.err != nil {
		return nil, s.err
	}

	defer s.reset()

	//get results
	rows, err := s.conn.Query(s.buffer.String(), s.args...)
	if err != nil {
		return nil, err
	}

	//create entities
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
	defer i.reset()
	if err := i.columnHandler.Validate(); err != nil {
		return err
	}
	row := i.conn.QueryRow(i.buffer.String(), i.columnHandler.ColumnValues(true)...)
	return i.columnHandler.Synchronize(row, i.entity)
}

type Delete struct {
	*Query
	args []interface{}
	err  error
}

func (d *Delete) Where(args map[string]interface{}) *Delete {
	d.args, d.err = addWhereCondition(args, &d.buffer, d.columnHandler)
	return d
}

func (d *Delete) Exec() (int64, error) {
	if d.err != nil {
		return 0, d.err
	}
	defer d.reset()
	res, err := d.conn.Exec(d.buffer.String(), d.args...)
	if err == nil {
		return res.RowsAffected()
	} else {
		return 0, err
	}
}

func addWhereCondition(whereCond map[string]interface{}, buffer *bytes.Buffer, columnHandler *ColumnHandler) ([]interface{}, error) {
	var args []interface{}
	var plcHdrIdx int

	//get sort list of fields
	fields := make([]string, 0, len(whereCond))
	for field := range whereCond {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	//render WHERE condition
	buffer.WriteString(" WHERE")
	for _, field := range fields {
		col, err := columnHandler.ColumnName(field)
		if err != nil {
			return args, err
		}
		if plcHdrIdx > 0 {
			buffer.WriteString(" AND")
		}
		plcHdrIdx++
		buffer.WriteString(fmt.Sprintf(" %s=$%d", col, plcHdrIdx))
		args = append(args, whereCond[field])
	}
	return args, nil
}
