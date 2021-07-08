package cluster

import (
	"fmt"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
)

type createdIntervalFilter struct {
	cluster  string
	interval time.Duration
}

func (rif *createdIntervalFilter) Filter(dbType db.Type, statusColHdr *db.ColumnHandler) (string, error) {
	clusterColName, err := statusColHdr.ColumnName("Cluster")
	if err != nil {
		return "", err
	}
	createdColName, err := statusColHdr.ColumnName("Created")
	if err != nil {
		return "", err
	}
	switch dbType {
	case db.Postgres:
		return fmt.Sprintf(`%s = '%s' AND %s >= NOW() - INTERVAL '%.0f SECOND'`,
			clusterColName, rif.cluster, createdColName, rif.interval.Seconds()), nil
	case db.SQLite:
		return fmt.Sprintf(`%s = '%s' AND %s >= DATETIME('now', '-%.0f SECONDS')`,
			clusterColName, rif.cluster, createdColName, rif.interval.Seconds()), nil
	default:
		return "", fmt.Errorf("Database type '%s' is not supported by this filter", dbType)
	}
}
