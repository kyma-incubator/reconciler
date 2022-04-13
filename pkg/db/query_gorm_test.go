package db

import (
	"github.com/kyma-incubator/reconciler/pkg/model"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func (s *DbTestSuite) TestQueryGorm() {
	s.NewConnection()
	t := s.T()

	// Used as tables for GORM Query
	type inventoryClusters struct{}
	type inventoryClusterConfigs struct{}
	type inventoryClusterConfigStatus struct{}

	testLogger := zap.NewExample().Sugar()
	defer func() {
		if err := testLogger.Sync(); err != nil {
			t.Logf("while flushing logs: %s", err)
		}
	}()

	type mockTable struct{}
	conn, err := s.NewConnection()
	require.NoError(t, err)

	q, err := NewQueryGorm(conn, &model.ClusterEntity{}, testLogger)
	require.NoError(t, err)

	t.Run("Insert", func(t *testing.T) {
		_, err = q.Insert(&inventoryClusters{})
		require.NoError(t, err)
		//require.Equal(t, "INSERT INTO mockTable (col_1, col_3) VALUES ($1, $2) RETURNING col_1, col_2, col_3", conn.query)
	})
}

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
		_, err = q.Insert(&mockTable{})
		require.NoError(t, err)
		//require.Equal(t, "INSERT INTO mockTable (col_1, col_3) VALUES ($1, $2) RETURNING col_1, col_2, col_3", conn.query)
	})
}
