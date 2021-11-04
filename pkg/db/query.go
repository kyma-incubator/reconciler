package db

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"
)

type Query struct {
	conn          Connection
	entity        DatabaseEntity
	columnHandler *ColumnHandler
	buffer        bytes.Buffer
	whereClause   bool
	Logger        *zap.SugaredLogger
}

func NewQuery(conn Connection, entity DatabaseEntity, logger *zap.SugaredLogger) (*Query, error) {
	columnHandler, err := NewColumnHandler(entity, conn, logger)
	if err != nil {
		return nil, err
	}
	return &Query{
		conn:          conn,
		entity:        entity,
		columnHandler: columnHandler,
		Logger:        logger,
	}, nil
}

func (q Query) String() string {
	return q.buffer.String()
}

func (q *Query) Select() *Select {
	q.buffer.WriteString(fmt.Sprintf("SELECT %s FROM %s", q.columnHandler.ColumnNamesCsv(false), q.entity.Table()))

	return &Select{q, []interface{}{}, nil}
}

func (q *Query) Insert() *Insert {
	colValPlcHdr, err := q.columnHandler.ColumnValuesPlaceholderCsv(true)

	q.buffer.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		q.entity.Table(), q.columnHandler.ColumnNamesCsv(true), colValPlcHdr, q.columnHandler.ColumnNamesCsv(false)))

	return &Insert{q, err}
}

func (q *Query) Delete() *Delete {
	q.buffer.WriteString(fmt.Sprintf("DELETE FROM %s", q.entity.Table()))

	return &Delete{q, []interface{}{}, nil}
}

func (q *Query) Update() *Update {
	colEntriesCsv, plcHdrCnt, err := q.columnHandler.columnEntriesCsvRenderer(true, true)

	q.buffer.WriteString(fmt.Sprintf("UPDATE %s SET %s",
		q.entity.Table(), colEntriesCsv))

	return &Update{q, []interface{}{}, plcHdrCnt, err}
}

// helper functions:
func (q *Query) reset() {
	q.buffer = bytes.Buffer{}
	q.whereClause = false
}

func (q *Query) addWhereCondition(whereCond map[string]interface{}, plcHdrOffset int) ([]interface{}, error) {
	var args []interface{}
	var plcHdrIdx int

	if len(whereCond) == 0 {
		return args, nil
	}

	//get sort list of fields
	fields := make([]string, 0, len(whereCond))
	for field := range whereCond {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	//render WHERE condition
	q.addWhere()
	for _, field := range fields {
		col, err := q.columnHandler.ColumnName(field)
		if err != nil {
			return args, err
		}
		if plcHdrIdx > 0 {
			q.buffer.WriteString(" AND")
		}
		plcHdrIdx++
		q.buffer.WriteString(fmt.Sprintf(" %s=$%d", col, plcHdrIdx+plcHdrOffset))
		args = append(args, whereCond[field])
	}
	return args, nil
}

func (q *Query) addWhereInCondition(field, subQuery string) error {
	colName, err := q.columnHandler.ColumnName(field)
	if err != nil {
		return err
	}
	q.addWhere()
	q.buffer.WriteString(fmt.Sprintf(" %s IN (%s)", colName, subQuery))
	return nil
}

func (q *Query) addWhere() {
	if q.whereClause {
		q.buffer.WriteString(" AND")
	} else {
		q.buffer.WriteString(" WHERE")
		q.whereClause = true
	}
}

// SELECT:
type Select struct {
	*Query
	args []interface{}
	err  error
}

func (s *Select) WhereRaw(stmt string, args ...interface{}) *Select {
	s.addWhere()
	s.buffer.WriteString(fmt.Sprintf(" (%s)", stmt))
	s.args = append(s.args, args...)
	return s
}

func (s *Select) Where(conds map[string]interface{}) *Select {
	args, err := s.addWhereCondition(conds, len(s.args))
	s.args = append(s.args, args...)
	s.err = err
	return s
}

func (s *Select) WhereIn(field, subQuery string, args ...interface{}) *Select {
	s.err = s.addWhereInCondition(field, subQuery)
	s.args = append(s.args, args...)
	return s
}

func (s *Select) GroupBy(args []string) *Select {
	if len(args) == 0 {
		return s
	}

	//get groupings
	var grouping []string
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
	var ordering []string
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

func (s *Select) Limit(limit int) *Select {
	s.buffer.WriteString(fmt.Sprintf(" LIMIT %d", limit))
	return s
}

func (s *Select) GetOne() (DatabaseEntity, error) {
	if s.err != nil {
		return nil, s.err
	}
	defer s.reset()
	row, err := s.conn.QueryRow(s.buffer.String(), s.args...)
	if err != nil {
		return nil, err
	}
	return s.entity, s.columnHandler.Unmarshal(row, s.entity)
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
	var result []DatabaseEntity
	for rows.Next() {
		entity := s.entity.New()
		colHdlr, err := NewColumnHandler(entity, s.conn, s.Query.Logger)
		if err != nil {
			return result, err
		}
		if err := colHdlr.Unmarshal(rows, entity); err != nil {
			return result, err
		}
		result = append(result, entity)
	}
	return result, nil
}

// INSERT:
type Insert struct {
	*Query
	err error
}

func (i *Insert) Exec() error {
	defer i.reset()
	if err := i.columnHandler.Validate(); err != nil {
		return err
	}
	colVals, err := i.columnHandler.ColumnValues(true)
	if err != nil {
		return err
	}
	row, err := i.conn.QueryRow(i.buffer.String(), colVals...)
	if err != nil {
		return err
	}
	return i.columnHandler.Unmarshal(row, i.entity)
}

// DELETE:
type Delete struct {
	*Query
	args []interface{}
	err  error
}

func (d *Delete) Where(args map[string]interface{}) *Delete {
	d.args, d.err = d.addWhereCondition(args, 0)
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
	}
	return 0, err
}

func (d *Delete) WhereIn(field, subQuery string, args ...interface{}) *Delete {
	d.err = d.addWhereInCondition(field, subQuery)
	d.args = args
	return d
}

// UPDATE:
type Update struct {
	*Query
	args              []interface{}
	placeholderOffset int
	err               error
}

func (u *Update) Where(args map[string]interface{}) *Update {
	u.args, u.err = u.addWhereCondition(args, u.placeholderOffset)
	return u
}

func (u *Update) ExecCount() (int64, error) {
	defer u.reset()
	colVals, err := u.colVals()
	if err != nil {
		return 0, err
	}

	rs, err := u.conn.Exec(u.buffer.String(), colVals...)
	if err != nil {
		return 0, err
	}

	return rs.RowsAffected()
}

func (u *Update) Exec() error {
	defer u.reset()
	colVals, err := u.colVals()
	if err != nil {
		return err
	}

	//finalize query by appending RETURNING
	u.buffer.WriteString(fmt.Sprintf(" RETURNING %s", u.columnHandler.ColumnNamesCsv(false)))

	row, err := u.conn.QueryRow(u.buffer.String(), colVals...)
	if err != nil {
		return err
	}
	return u.columnHandler.Unmarshal(row, u.entity)
}

func (u *Update) colVals() ([]interface{}, error) {
	if err := u.columnHandler.Validate(); err != nil {
		return nil, err
	}

	colVals, err := u.columnHandler.ColumnValues(true)
	if err != nil {
		return nil, err
	}

	if len(u.args) > 0 {
		colVals = append(colVals, u.args...)
	}

	return colVals, nil
}
