package gardener

import (
	"fmt"
	"testing"

	gardener_types "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/stretchr/testify/assert"
)

const (
	gardenerNamespace = "default"
	runtimeId         = "runtimeId"
	tenant            = "tenant"
	operationId       = "operationId"
	clusterName       = "test-cluster"
	region            = "westeurope"
	purpose           = "production"

	auditLogsPolicyCMName = "audit-logs-policy"
)

//
//func TestGardenerProvisioner_ProvisionCluster(t *testing.T) {
//	clientset := fake.NewSimpleClientset()
//
//	gcpGardenerConfig, err := NewGCPGardenerConfig(&gqlschema.GCPProviderConfigInput{
//		Zones: []string{"zone-1"},
//	})
//	require.NoError(t, err)
//
//	maintWindowConfigPath := filepath.Join("testdata", "maintwindow.json")
//
//	cluster := newClusterConfig("test-cluster", nil, gcpGardenerConfig, region, purpose)
//
//	t.Run("should start provisioning", func(t *testing.T) {
//		// given
//		shootClient := clientset.CoreV1beta1().Shoots(gardenerNamespace)
//
//		provisionerClient := NewProvisioner(gardenerNamespace, shootClient, nil, auditLogsPolicyCMName, maintWindowConfigPath)
//
//		// when
//		apperr := provisionerClient.ProvisionCluster(cluster, operationId)
//		require.NoError(t, apperr)
//
//		// then
//		shoot, err := shootClient.Get(context.Background(), clusterName, v1.GetOptions{})
//		require.NoError(t, err)
//		assertAnnotation(t, shoot, operationIDAnnotation, operationId)
//		assertAnnotation(t, shoot, runtimeIDAnnotation, runtimeId)
//		assertAnnotation(t, shoot, legacyOperationIDAnnotation, operationId)
//		assertAnnotation(t, shoot, legacyRuntimeIDAnnotation, runtimeId)
//		assert.Equal(t, "", shoot.Labels[model.SubAccountLabel])
//
//		require.NotNil(t, shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig)
//		require.NotNil(t, shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy)
//		require.NotNil(t, shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef)
//		require.NotNil(t, shoot.Spec.Maintenance.TimeWindow)
//		assert.Equal(t, auditLogsPolicyCMName, shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name)
//	})
//}
//
//func newClusterConfig(name string, subAccountID *string, providerConfig model.GardenerProviderConfig, region string, purpose string) model.Cluster {
//	return model.Cluster{
//		ID:           runtimeId,
//		Tenant:       tenant,
//		SubAccountId: subAccountID,
//		ClusterConfig: model.GardenerConfig{
//			ID:                     "id",
//			ClusterID:              runtimeId,
//			Name:                   name,
//			ProjectName:            "project-name",
//			KubernetesVersion:      "1.16",
//			VolumeSizeGB:           util.IntPtr(50),
//			DiskType:               util.StringPtr("standard"),
//			MachineType:            "n1-standard-4",
//			Provider:               "gcp",
//			TargetSecret:           "secret",
//			Region:                 region,
//			Purpose:                util.StringPtr(purpose),
//			WorkerCidr:             "10.10.10.10",
//			AutoScalerMin:          1,
//			AutoScalerMax:          5,
//			MaxSurge:               25,
//			MaxUnavailable:         1,
//			GardenerProviderConfig: providerConfig,
//		},
//	}
//}

//func TestGardenerProvisioner_ClusterPurpose(t *testing.T) {
//	clientset_A := fake.NewSimpleClientset()
//	clientset_B := fake.NewSimpleClientset()
//
//	gcpGardenerConfig, err := model.NewGCPGardenerConfig(&gqlschema.GCPProviderConfigInput{Zones: []string{"zone-1"}})
//	require.NoError(t, err)
//	cluster_A := newClusterConfig(clusterName, nil, gcpGardenerConfig, region, "")
//	cluster_B := newClusterConfig(clusterName, nil, gcpGardenerConfig, region, purpose)
//
//	maintWindowConfigPath := filepath.Join("testdata", "maintwindow.json")
//
//	t.Run("should start provisioning with 2 clusters with different purpose", func(t *testing.T) {
//		shootClient_A := clientset_A.CoreV1beta1().Shoots(gardenerNamespace)
//		provisionerClient_A := NewProvisioner(gardenerNamespace, shootClient_A, nil, auditLogsPolicyCMName, maintWindowConfigPath)
//
//		shootClient_B := clientset_B.CoreV1beta1().Shoots(gardenerNamespace)
//		provisionerClient_B := NewProvisioner(gardenerNamespace, shootClient_B, nil, auditLogsPolicyCMName, maintWindowConfigPath)
//
//		//when
//		apperr_A := provisionerClient_A.ProvisionCluster(cluster_A, operationId)
//		require.NoError(t, apperr_A)
//		apperr_B := provisionerClient_B.ProvisionCluster(cluster_B, operationId)
//		require.NoError(t, apperr_B)
//
//		//then
//		shoot_A, err := shootClient_A.Get(context.Background(), clusterName, v1.GetOptions{})
//		require.NoError(t, err)
//		shoot_B, err := shootClient_B.Get(context.Background(), clusterName, v1.GetOptions{})
//		require.NoError(t, err)
//		assert.NotEqual(t, shoot_A.Spec.Maintenance.TimeWindow, shoot_B.Spec.Maintenance.TimeWindow)
//	})
//}

func assertAnnotation(t *testing.T, shoot *gardener_types.Shoot, name, value string) {
	annotations := shoot.Annotations
	if annotations == nil {
		t.Errorf("annotations are nil, expected annotation: %s, value: %s", name, value)
		return
	}

	val, found := annotations[name]
	if !found {
		t.Errorf("annotation not found, expected annotation: %s, value: %s", name, value)
		return
	}

	assert.Equal(t, value, val, fmt.Sprintf("invalid value for %s annotation", name))
}
