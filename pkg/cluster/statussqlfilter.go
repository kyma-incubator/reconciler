package cluster

import (
	"fmt"
	"strings"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

const (
	CreatedAtColumnName       = "Created"
	StatusCreatedAtColumnName = "StatusCreatedAt"
	StatusColumnName          = "Status"
	RuntimeIDColumnName       = "RuntimeID"
	ConfigIDColumnName        = "ConfigID"
)

type statusSQLFilter interface {
	Filter(dbType db.Type, colHdr *db.ColumnHandler) (string, error)
}

type statusFilter struct {
	allowedStatuses []model.Status
}

func (sf *statusFilter) Filter(_ db.Type, statusColHdr *db.ColumnHandler) (string, error) {
	statusColName, err := statusColHdr.ColumnName(StatusColumnName)
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
	statusColName, err := statusColHdr.ColumnName(StatusColumnName)
	if err != nil {
		return "", err
	}
	createdColName, err := statusColHdr.ColumnName(StatusCreatedAtColumnName)
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

type createdIntervalFilter struct {
	runtimeID string
	interval  time.Duration
}

func (rif *createdIntervalFilter) Filter(dbType db.Type, statusColHdr *db.ColumnHandler) (string, error) {
	runtimeIDColName, err := statusColHdr.ColumnName(RuntimeIDColumnName)
	if err != nil {
		return "", err
	}
	createdColName, err := statusColHdr.ColumnName(CreatedAtColumnName)
	if err != nil {
		return "", err
	}
	switch dbType {
	case db.Postgres:
		return fmt.Sprintf(`%s = '%s' AND %s >= NOW() - INTERVAL '%.0f SECOND'`,
			runtimeIDColName, rif.runtimeID, createdColName, rif.interval.Seconds()), nil
	case db.SQLite:
		return fmt.Sprintf(`%s = '%s' AND %s >= DATETIME('now', '-%.0f SECONDS')`,
			runtimeIDColName, rif.runtimeID, createdColName, rif.interval.Seconds()), nil
	default:
		return "", fmt.Errorf("database type '%s' is not supported by this filter", dbType)
	}
}

type runtimeIDFilter struct {
	runtimeID string
}

func (r *runtimeIDFilter) Filter(_ db.Type, statusColHdr *db.ColumnHandler) (string, error) {
	runtimeIDColName, err := statusColHdr.ColumnName(RuntimeIDColumnName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s = '%s'", runtimeIDColName, r.runtimeID), nil
}

type configIDFilter struct {
	configID int64
}

func (r *configIDFilter) Filter(_ db.Type, statusColHdr *db.ColumnHandler) (string, error) {
	configIDColName, err := statusColHdr.ColumnName(ConfigIDColumnName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s = '%v'", configIDColName, r.configID), nil
}
