package cluster

import (
	"database/sql"
	"github.com/kyma-incubator/reconciler/pkg/db"
	"io/ioutil"
	"os"
	"time"

	file "github.com/kyma-incubator/reconciler/pkg/files"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
)

const envVarKubeconfig = "KUBECONFIG"

type MockInventory struct {
	ClustersToReconcileResult []*State
	ClustersNotReadyResult    []*State
	GetResult                 *State
	GetLatestResult           *State
	GetAllResult              []*State
	CreateOrUpdateResult      *State
	MarkForDeletionResult     *State
	DeleteResult              error
	UpdateStatusResult        *State
	ChangesResult             []*StatusChange
	DBStatsResult             *sql.DBStats
	RetriesCount              int
}

func (i *MockInventory) WithTx(_ *db.TxConnection) (Inventory, error) {
	return i, nil
}

func (i *MockInventory) DBStats() *sql.DBStats {
	return i.DBStatsResult
}

func (i *MockInventory) CreateOrUpdate(_ int64, _ *keb.Cluster) (*State, error) {
	return i.CreateOrUpdateResult, nil
}

func (i *MockInventory) UpdateStatus(_ *State, _ model.Status) (*State, error) {
	return i.UpdateStatusResult, nil
}

func (i *MockInventory) MarkForDeletion(_ string) (*State, error) {
	return i.MarkForDeletionResult, nil
}

func (i *MockInventory) Delete(_ string) error {
	return i.DeleteResult
}

func (i *MockInventory) Get(_ string, _ int64) (*State, error) {
	return i.GetResult, nil
}

func (i *MockInventory) GetLatest(_ string) (*State, error) {
	return i.GetLatestResult, nil
}

func (i *MockInventory) GetAll() ([]*State, error) {
	return i.GetAllResult, nil
}

func (i *MockInventory) ClustersToReconcile(_ time.Duration) ([]*State, error) {
	return i.ClustersToReconcileResult, nil
}

func (i *MockInventory) ClustersNotReady() ([]*State, error) {
	return i.ClustersNotReadyResult, nil
}

func (i *MockInventory) StatusChanges(_ string, _ time.Duration) ([]*StatusChange, error) {
	return i.ChangesResult, nil
}

type MockKubeconfigProvider struct {
	KubeconfigResult string
}

func (kp *MockKubeconfigProvider) Get() (string, error) {
	if kp.KubeconfigResult == "" && file.Exists(os.Getenv(envVarKubeconfig)) {
		kubeCfg, err := ioutil.ReadFile(os.Getenv(envVarKubeconfig))
		if err != nil {
			return "", err
		}
		return string(kubeCfg), nil
	}
	return kp.KubeconfigResult, nil
}

func (i *MockInventory) CountRetries(_ string, _ int64, _ int, _ ...model.Status) (int, error) {
	return i.RetriesCount, nil
}
