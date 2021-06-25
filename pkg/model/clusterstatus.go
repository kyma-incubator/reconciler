package model

import (
	"fmt"
	"strings"
)

const (
	InstallPending   ClusterStatus = "install_pending"
	UpgradePending   ClusterStatus = "upgrade_pending"
	ReconcilePending ClusterStatus = "reconcile_pending"
	Installed        ClusterStatus = "installed"
	Installing       ClusterStatus = "installing"
	Upgrading        ClusterStatus = "upgrading"
	Reconciling      ClusterStatus = "reconciling"
	UpgradeFailed    ClusterStatus = "upgrade_failed"
	InstallFailed    ClusterStatus = "install_failed"
	Error            ClusterStatus = "error"
)

type ClusterStatus string

func NewClusterStatus(clusterStatus string) (ClusterStatus, error) {
	switch strings.ToLower(clusterStatus) {
	case string(InstallPending):
		return InstallPending, nil
	case string(UpgradePending):
		return UpgradePending, nil
	case string(ReconcilePending):
		return ReconcilePending, nil
	case string(Installed):
		return Installed, nil
	case string(Installing):
		return Installing, nil
	case string(Upgrading):
		return Upgrading, nil
	case string(Reconciling):
		return Reconciling, nil
	case string(UpgradeFailed):
		return UpgradeFailed, nil
	case string(InstallFailed):
		return InstallFailed, nil
	case string(Error):
		return Error, nil
	default:
		return "", fmt.Errorf("ClusterStatus '%s' is unknown", clusterStatus)
	}
}
