package cluster

import (
	"fmt"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

type statusSQLFilter interface {
	Filter(dbType db.Type, colHdr *db.ColumnHandler) (string, error)
}

type statusFilter struct {
	allowedStatuses []model.Status
}

func (sf *statusFilter) Filter(_ db.Type, statusColHdr *db.ColumnHandler) (string, error) {
	statusColName, err := statusColHdr.ColumnName("Status")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s IN ('%s')", statusColName, strings.Join(sf.statusesToStrings(), "','")), nil
}

func (sf *statusFilter) statusesToStrings() []string {
	var result []string
	for _, status := range sf.allowedStatuses {
		result = append(result, string(status))
	}
	return result
}

type reconcileIntervalFilter struct {
	reconcileInterval time.Duration
}

func (rif *reconcileIntervalFilter) Filter(dbType db.Type, statusColHdr *db.ColumnHandler) (string, error) {
	statusColName, err := statusColHdr.ColumnName("Status")
	if err != nil {
		return "", err
	}
	createdColName, err := statusColHdr.ColumnName("Created")
	if err != nil {
		return "", err
	}
	switch dbType {
	case db.Postgres:
		return fmt.Sprintf(`%s IN ('%s', '%s', '%s') AND %s <= NOW() - INTERVAL '%.0f SECOND'`,
			statusColName, model.ClusterStatusReady, model.ClusterStatusReconcileErrorRetryable, model.ClusterStatusDeleteErrorRetryable, createdColName, rif.reconcileInterval.Seconds()), nil
	case db.SQLite:
		return fmt.Sprintf(`%s IN ('%s', '%s', '%s') AND %s <= DATETIME('now', '-%.0f SECONDS')`,
			statusColName, model.ClusterStatusReady, model.ClusterStatusReconcileErrorRetryable, model.ClusterStatusDeleteErrorRetryable, createdColName, rif.reconcileInterval.Seconds()), nil
	default:
		return "", fmt.Errorf("database type '%s' is not supported by this filter", dbType)
	}
}
