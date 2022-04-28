package db

import (
	"gorm.io/gorm/clause"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestQueryGorm(t *testing.T) {
	testLogger := zap.NewExample().Sugar()
	defer func() {
		if err := testLogger.Sync(); err != nil {
			t.Logf("while flushing logs: %s", err)
		}
	}()

	type mockTable struct{}
	conn := &MockConnection{}
	q, err := NewQueryGorm(conn, &MockDbEntity{
		Col1: "dummy",
	}, testLogger)
	require.NoError(t, err)

	t.Run("Insert", func(t *testing.T) {
		ret, err := q.Insert(mockTable{})
		require.NoError(t, err)
		require.Equal(t, &MockDbEntity{Col1: "col1", Col2: true, Col3: 3}, ret)
	})

	t.Run("GetOne", func(t *testing.T) {
		whereCond := map[string]interface{}{
			"col1": "col1",
		}
		ret, err := q.GetOne(whereCond, "col1 desc", mockTable{})
		require.NoError(t, err)
		require.Equal(t, &MockDbEntity{Col1: "col1", Col2: true, Col3: 3}, ret)
	})

	t.Run("Get column names as CSV", func(t *testing.T) {
		require.Equal(t, "col_1, col_2, col_3", q.ColumnNamesCsv(false))
		require.Equal(t, "col_1, col_3", q.ColumnNamesCsv(true))
	})

	t.Run("Get column names as slice", func(t *testing.T) {
		require.Equal(t, []string{"col_1", "col_2", "col_3"}, q.ColumnNamesSlice(false))
		require.Equal(t, []string{"col_1", "col_3"}, q.ColumnNamesSlice(true))
	})

	t.Run("Get column names as gorm clause", func(t *testing.T) {
		require.Equal(t, []clause.Column{{Name: "col_1"}, {Name: "col_2"}, {Name: "col_3"}}, q.ColumnNamesGormClause(false))
		require.Equal(t, []clause.Column{{Name: "col_1"}, {Name: "col_3"}}, q.ColumnNamesGormClause(true))
	})

	t.Run("Get values", func(t *testing.T) {
		require.Equal(t, 3, len(q.columnHandler.columns))

		colValsAll, err := q.ColumnValues(true)
		require.NoError(t, err)
		//don't test for encrypted value in col3 (enc-values are unique and cannot be reproduced/predicted)
		require.Len(t, colValsAll, 2)
		require.Contains(t, colValsAll, "dummy")

		colValsAll, err = q.ColumnValues(false)
		require.NoError(t, err)
		//don't test for encrypted value in col3 (enc-values are unique and cannot be reproduced/predicted)
		require.Len(t, colValsAll, 3)
		require.Contains(t, colValsAll, "dummy")
		require.Contains(t, colValsAll, false)
	})

	t.Run("Get ColumnMap", func(t *testing.T) {
		columnMap := q.ColumnMap(true)
		//don't test for encrypted value in col3 (enc-values are unique and cannot be reproduced/predicted)
		require.Len(t, columnMap, 2)
		val, ok := columnMap["col_1"]
		require.True(t, ok)
		require.Equal(t, val, "dummy")

		columnMap = q.ColumnMap(false)
		//don't test for encrypted value in col3 (enc-values are unique and cannot be reproduced/predicted)
		require.Len(t, columnMap, 3)
		val, ok = columnMap["col_1"]
		require.True(t, ok)
		require.Equal(t, val, "dummy")
		val, ok = columnMap["col_2"]
		require.True(t, ok)
		require.Equal(t, false, val)

	})
}
