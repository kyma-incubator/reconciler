package db

import (
	"go.uber.org/zap"
	gormPg "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type QueryGorm struct {
	Conn          Connection
	gormDB        *gorm.DB
	columnHandler *ColumnHandler
	DbEntity      DatabaseEntity
}

func NewQueryGorm(conn Connection, entity DatabaseEntity, logger *zap.SugaredLogger) (*QueryGorm, error) {
	columnHandler, err := NewColumnHandler(entity, conn, logger)
	if err != nil {
		return &QueryGorm{}, err
	}
	gormDB, err := gorm.Open(gormPg.New(gormPg.Config{Conn: conn.DB()}), &gorm.Config{DryRun: true})
	if err != nil {
		return &QueryGorm{}, err
	}
	return &QueryGorm{
		Conn:          conn,
		gormDB:        gormDB,
		columnHandler: columnHandler,
		DbEntity:      entity,
	}, nil
}

func (q *QueryGorm) Query() *gorm.DB {
	return q.gormDB.Session(&gorm.Session{DryRun: true})
}

func (q *QueryGorm) Insert(dbTable interface{}) (DatabaseEntity, error) {
	insertSQL := q.Query().Model(dbTable).Clauses(clause.Returning{Columns: q.ColumnNamesGormClause(false)}).Create(q.ColumnMap(true))
	row, err := q.Conn.QueryRow(GetString(insertSQL), GetVars(insertSQL)...)
	if err != nil {
		return nil, err
	}
	if err := q.Unmarshal(row); err != nil {
		return nil, err
	}
	return q.DbEntity, nil
}

func (q *QueryGorm) GetOne(whereCond map[string]interface{}, order string, dest interface{}) (DatabaseEntity, error) {
	clusterStatusEntitySQL := q.Query().Select("*").
		Where(whereCond).
		Order(order).Find(dest)
	clusterEntity, err := q.Conn.QueryRow(GetString(clusterStatusEntitySQL), GetVars(clusterStatusEntitySQL)...)
	if err != nil {
		return nil, err
	}
	if err := q.Unmarshal(clusterEntity); err != nil {
		return nil, err
	}
	return q.DbEntity, nil
}

func (q *QueryGorm) Unmarshal(row DataRow) error {
	return q.columnHandler.Unmarshal(row, q.DbEntity)
}

func (q *QueryGorm) ColumnNamesCsv(onlyWriteable bool) string {
	return q.columnHandler.ColumnNamesCsv(onlyWriteable)
}

func (q *QueryGorm) ColumnNamesSlice(onlyWritebale bool) []string {
	return q.columnHandler.ColumnNamesSlice(onlyWritebale)
}

func (q *QueryGorm) ColumnNamesGormClause(onlyWritebale bool) []clause.Column {
	return q.columnHandler.ColumnNamesGormClause(onlyWritebale)
}

func (q *QueryGorm) ColumnValues() ([]interface{}, error) {
	return q.columnHandler.ColumnValues(true)
}

func (q *QueryGorm) ColumnMap(onlyWriteable bool) map[string]interface{} {
	res := make(map[string]interface{})
	values, _ := q.ColumnValues()
	for i, col := range q.ColumnNamesSlice(onlyWriteable) {
		res[col] = values[i] // Hier panict er
	}
	return res
}

func GetString(gormDB *gorm.DB) string {
	return gormDB.Statement.SQL.String()
}

func GetVars(gormDB *gorm.DB) []interface{} {
	return gormDB.Statement.Vars
}
