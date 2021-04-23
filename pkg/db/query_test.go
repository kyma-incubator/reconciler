package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuery(t *testing.T) {
	conn := &MockConnection{}
	q, err := NewQuery(conn, &MockDbEntity{})
	require.NoError(t, err)

	t.Run("Insert", func(t *testing.T) {
		err = q.Insert().Exec()
		require.NoError(t, err)
		require.Equal(t, "INSERT INTO mockTable (col_1, col_2, col_3) VALUES ($1, $2, $3) RETURNING col_1, col_2, col_3", conn.query)
	})

	t.Run("Select", func(t *testing.T) {
		_, err := q.Select().
			Where(map[string]interface{}{"Col1": "col1Value", "Col2": true}).
			GroupBy([]string{"Col2"}).
			OrderBy(map[string]string{"Col3": "DESC"}).
			GetOne()
		require.NoError(t, err)
		require.Equal(t, "SELECT col_1, col_2, col_3 FROM mockTable WHERE col_1=$1 AND col_2=$2 GROUP BY col_2 ORDER BY col_3 DESC", conn.query)
		require.Equal(t, []interface{}{"col1Value", true}, conn.args)
	})

	t.Run("Delete", func(t *testing.T) {
		affected, err := q.Delete().
			Where(map[string]interface{}{"Col1": "col1Value", "Col2": true}).
			Exec()
		require.NoError(t, err)
		require.Equal(t, MockRowsAffected, affected)
		require.Equal(t, "DELETE FROM mockTable WHERE col_1=$1 AND col_2=$2", conn.query)
		require.Equal(t, []interface{}{"col1Value", true}, conn.args)
	})
}
