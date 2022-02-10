package test

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/stretchr/testify/require"
	"testing"
)

func NewCluster(t *testing.T, runtimeID string, clusterVersion uint64, newConfigVersion bool, clusterType Cluster) *keb.Cluster {
	runtimeID += uuid.NewString()
	cluster := &keb.Cluster{}
	err := json.Unmarshal(clusterType, cluster)
	require.NoError(t, err)
	cluster.RuntimeID = fmt.Sprintf("runtime%s", runtimeID)
	cluster.RuntimeInput.Name = fmt.Sprintf("runtimeName%d", clusterVersion)
	cluster.RuntimeInput.Description = fmt.Sprintf("For test: %s", t.Name())
	cluster.Metadata.GlobalAccountID = fmt.Sprintf("globalAccountId%d", clusterVersion)
	cluster.Kubeconfig = "fake kubeconfig"

	suffix := getSuffix(newConfigVersion, clusterVersion)
	cluster.KymaConfig.Profile = fmt.Sprintf("kymaProfile%s", suffix)
	cluster.KymaConfig.Version = fmt.Sprintf("kymaVersion%s", suffix)

	return cluster
}

func getSuffix(newConfigVersion bool, clusterVersion uint64) string {
	var suffix string
	if newConfigVersion {
		suffix = fmt.Sprintf("%d_%s", clusterVersion, uuid.NewString()) //leads always to a new cluster-config entity
	} else {
		suffix = fmt.Sprintf("%d", clusterVersion)
	}
	return suffix
}

func NewClusterFromExisting(existingCluster keb.Cluster, newClusterVersion uint64, newConfigVersion bool) *keb.Cluster {
	updatedCluster := existingCluster
	updatedCluster.RuntimeInput.Name = fmt.Sprintf("runtimeName%d", newClusterVersion)
	updatedCluster.Metadata.GlobalAccountID = fmt.Sprintf("globalAccountId%d", newClusterVersion)
	suffix := getSuffix(newConfigVersion, newClusterVersion)
	updatedCluster.KymaConfig.Profile = fmt.Sprintf("kymaProfile%s", suffix)
	updatedCluster.KymaConfig.Version = fmt.Sprintf("kymaVersion%s", suffix)
	return &updatedCluster
}
