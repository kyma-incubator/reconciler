package model

import (
	"context"
	"fmt"
	log "github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/kyma-incubator/reconciler/pkg/scheduler/config"
	"github.com/kyma-incubator/reconciler/pkg/test"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	entity1 *ClusterConfigurationEntity
	entity2 *ClusterConfigurationEntity
	equal   bool
}

type isKubeconfigTestCase struct {
	kubeconfig     string
	expectedResult bool
}

func TestIsKubeconfig(t *testing.T) {
	testCases := []*isKubeconfigTestCase{
		{
			kubeconfig:     "",
			expectedResult: false,
		},
		{
			kubeconfig:     "abc",
			expectedResult: false,
		},
		{
			kubeconfig:     "{}",
			expectedResult: false,
		},
		{
			kubeconfig:     "---",
			expectedResult: false,
		},
		{
			kubeconfig: func() string {
				return string(test.ReadFile(t, "test/kubeconfig_valid.yaml"))
			}(),
			expectedResult: true,
		},
		{
			kubeconfig: func() string {
				return string(test.ReadFile(t, "test/kubeconfig_invalid.yaml"))
			}(),
			expectedResult: false,
		},
	}
	for _, tc := range testCases {
		result := isKubeconfig(tc.kubeconfig)
		if tc.expectedResult {
			require.True(t, result, fmt.Sprintf("Expected valid kubeconfig  when parsing string '%s'", tc.kubeconfig))
		} else {
			require.False(t, result, fmt.Sprintf("Expected invalid kubeconfig when parsing string '%s'", tc.kubeconfig))
		}
	}
}

func TestClusterConfigEntity(t *testing.T) {
	t.Parallel()

	t.Run("Validate Equal", func(t *testing.T) {
		testCases := []*testCase{
			{
				entity1: &ClusterConfigurationEntity{
					Version:        1,
					RuntimeID:      "1234",
					ClusterVersion: 1,
					KymaVersion:    "1.2.3",
					KymaProfile:    "prod",
					Components:     nil,
					Administrators: nil,
					Contract:       1,
				},
				entity2: &ClusterConfigurationEntity{
					Version:        1,
					RuntimeID:      "1234",
					ClusterVersion: 1,
					KymaVersion:    "1.2.3",
					KymaProfile:    "prod",
					Components:     nil,
					Administrators: nil,
					Contract:       1,
				},
				equal: true,
			},
			{
				entity1: &ClusterConfigurationEntity{
					Version:        1,
					RuntimeID:      "1234",
					ClusterVersion: 1,
					KymaVersion:    "1.2.3",
					KymaProfile:    "prod",
					Components:     nil,
					Administrators: nil,
					Contract:       1,
				},
				entity2: &ClusterConfigurationEntity{
					Version:        1,
					RuntimeID:      "1234",
					ClusterVersion: 1,
					KymaVersion:    "1.2.3",
					KymaProfile:    "prod",
					Components:     []*keb.Component{},
					Administrators: []string{},
					Contract:       1,
				},
				equal: false,
			},
			{
				entity1: &ClusterConfigurationEntity{
					Version:        1,
					RuntimeID:      "1234",
					ClusterVersion: 1,
					KymaVersion:    "1.2.3",
					KymaProfile:    "prod",
					Components: []*keb.Component{
						crdComponent,
					},
					Administrators: []string{"admin1"},
					Contract:       1,
				},
				entity2: &ClusterConfigurationEntity{
					Version:        1,
					RuntimeID:      "1234",
					ClusterVersion: 1,
					KymaVersion:    "1.2.3",
					KymaProfile:    "prod",
					Components: []*keb.Component{
						{
							URL:           "http://x.y.z",
							Component:     "comp",
							Configuration: nil,
							Namespace:     "default",
							Version:       "1.2.3",
						},
					},
					Administrators: []string{"admin2"},
					Contract:       1,
				},
				equal: false,
			},
			{
				entity1: &ClusterConfigurationEntity{
					Version:        1,
					RuntimeID:      "1234",
					ClusterVersion: 1,
					KymaVersion:    "1.2.3",
					KymaProfile:    "prod",
					Components: []*keb.Component{
						crdComponent,
					},
					Administrators: []string{"admin1"},
					Contract:       1,
				},
				entity2: &ClusterConfigurationEntity{
					Version:        1,
					RuntimeID:      "1234",
					ClusterVersion: 1,
					KymaVersion:    "1.2.3",
					KymaProfile:    "prod",
					Components: []*keb.Component{
						crdComponent,
					},
					Administrators: []string{"admin1"},
					Contract:       1,
				},
				equal: true,
			},
		}

		for _, testCase := range testCases {
			if testCase.equal {
				require.True(t, testCase.entity1.Equal(testCase.entity2))
			} else {
				require.False(t, testCase.entity1.Equal(testCase.entity2))
			}
		}
	})

}

func TestReconciliationSequence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		preComps             [][]string
		entity               *ClusterConfigurationEntity
		reconciliationStatus Status
		expected             *ReconciliationSequence
		err                  error
	}{
		{
			name:                 "Components and single pre-components",
			preComps:             [][]string{{"Pre1"}, {"Pre2"}},
			reconciliationStatus: ClusterStatusReconciling,
			entity: &ClusterConfigurationEntity{
				Components: []*keb.Component{
					{
						Component: "Pre1",
					},
					{
						Component: "Pre2",
					},
					{
						Component: "Comp1",
					},
					{
						Component: "Comp2",
					},
				},
			},
			expected: &ReconciliationSequence{
				Queue: [][]*keb.Component{
					{
						crdComponent,
					},
					{
						{
							Component: "Pre1",
						},
					},
					{
						{
							Component: "Pre2",
						},
					},
					{
						{
							Component: "Comp1",
						},
						{
							Component: "Comp2",
						},
					},
				},
			},
			err: nil,
		},
		{
			name:                 "Component and Pre-Component with ClusterStatusDeleting",
			preComps:             [][]string{{"Pre"}},
			reconciliationStatus: ClusterStatusDeleting,
			entity: &ClusterConfigurationEntity{
				Components: []*keb.Component{
					{
						Component: "Pre",
					},
					{
						Component: "Comp",
					},
				},
			},
			expected: &ReconciliationSequence{
				Queue: [][]*keb.Component{
					{
						crdComponent,
					},
					{
						cleanupComponent,
					},
					{
						{
							Component: "Pre",
						},
					},
					{
						{
							Component: "Comp",
						},
					},
				},
			},
			err: nil,
		},
		{
			name:                 "Components and multiple pre components",
			preComps:             [][]string{{"Pre1.1", "Pre1.2"}, {"Pre2"}, {"Pre3.1", "Pre3.2"}},
			reconciliationStatus: ClusterStatusReconciling,
			entity: &ClusterConfigurationEntity{
				Components: []*keb.Component{
					{
						Component: "Pre1.1",
					},
					{
						Component: "Pre1.2",
					},
					{
						Component: "Pre2",
					},
					{
						Component: "Pre3.1",
					},
					{
						Component: "Pre3.2",
					},
					{
						Component: "Comp1",
					},
					{
						Component: "Comp2",
					},
				},
			},
			expected: &ReconciliationSequence{
				Queue: [][]*keb.Component{
					{
						crdComponent,
					},
					{
						{
							Component: "Pre1.1",
						},
						{
							Component: "Pre1.2",
						},
					},
					{
						{
							Component: "Pre2",
						},
					},
					{
						{
							Component: "Pre3.1",
						},
						{
							Component: "Pre3.2",
						},
					},
					{
						{
							Component: "Comp1",
						},
						{
							Component: "Comp2",
						},
					},
				},
			},
			err: nil,
		},
		{
			name:                 "Components and multiple pre-components with missing pre-components",
			preComps:             [][]string{{"Pre1.1", "Pre1.2"}, {"Pre2"}, {"Pre3.1", "Pre3.2"}},
			reconciliationStatus: ClusterStatusReconciling,
			entity: &ClusterConfigurationEntity{
				Components: []*keb.Component{
					{
						Component: "Pre1.1",
					},
					{
						Component: "Pre3.1",
					},
					{
						Component: "Pre3.2",
					},
					{
						Component: "Comp1",
					},
					{
						Component: "Comp2",
					},
				},
			},
			expected: &ReconciliationSequence{
				Queue: [][]*keb.Component{
					{
						crdComponent,
					},
					{
						{
							Component: "Pre1.1",
						},
					},
					{
						{
							Component: "Pre3.1",
						},
						{
							Component: "Pre3.2",
						},
					},
					{
						{
							Component: "Comp1",
						},
						{
							Component: "Comp2",
						},
					},
				},
			},
			err: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.entity.GetReconciliationSequence(&ReconciliationSequenceConfig{
				PreComponents:        tc.preComps,
				DeleteStrategy:       "system",
				ReconciliationStatus: tc.reconciliationStatus,
			})
			for idx, expected := range tc.expected.Queue {
				require.ElementsMatch(t, result.Queue[idx], expected)
			}
		})
	}
}

func TestReconciliationSequenceWithMigratedComponents(t *testing.T) {
	test.IntegrationTest(t)

	k8sClient, err := kubernetes.NewKubernetesClient(test.ReadKubeconfig(t), log.NewLogger(true), nil)
	require.NoError(t, err)
	_, err = k8sClient.Deploy(context.Background(), test.ReadManifest(t, "crd_comp1.yaml"), "default")
	require.NoError(t, err)
	_, err = k8sClient.Deploy(context.Background(), test.ReadManifest(t, "crd_pre2.yaml"), "default")
	require.NoError(t, err)

	defer func() {
		_, err = k8sClient.Delete(context.Background(), test.ReadManifest(t, "crd_comp1.yaml"), "default")
		require.NoError(t, err)
		_, err = k8sClient.Delete(context.Background(), test.ReadManifest(t, "crd_pre2.yaml"), "default")
		require.NoError(t, err)
	}()

	tests := []struct {
		name                 string
		preComps             [][]string
		componentCRDs        map[string]config.ComponentCRD
		entity               *ClusterConfigurationEntity
		reconciliationStatus Status
		expected             *ReconciliationSequence
		err                  error
	}{
		{
			name:     "With migrated and non-migrated components",
			preComps: [][]string{{"Pre1"}, {"Pre2"}},
			componentCRDs: map[string]config.ComponentCRD{
				"PreX": {
					Group:   "reconciler.kyma-project.io",
					Version: "v1beta1",
					Kind:    "unknown1",
				},
				"Pre2": {
					Group:   "reconciler.kyma-project.io",
					Version: "v2",
					Kind:    "pres2",
				},
				"Comp1": {
					Group:   "reconciler.kyma-project.io",
					Version: "v1",
					Kind:    "comps1",
				},
				"Comp2": {
					Group:   "reconciler.kyma-project.io",
					Version: "v2beta2",
					Kind:    "unknown2",
				},
			},
			reconciliationStatus: ClusterStatusReconciling,
			entity: &ClusterConfigurationEntity{
				Components: []*keb.Component{
					{
						Component: "Pre1",
					},
					{
						Component: "Pre2",
					},
					{
						Component: "Comp1",
					},
					{
						Component: "Comp2",
					},
				},
			},
			expected: &ReconciliationSequence{
				Queue: [][]*keb.Component{
					{
						crdComponent,
					},
					{
						{
							Component: "Pre1",
						},
					},
					{
						{
							Component: "Comp2",
						},
					},
				},
			},
			err: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.entity.GetReconciliationSequence(&ReconciliationSequenceConfig{
				PreComponents:        tc.preComps,
				DeleteStrategy:       "system",
				ReconciliationStatus: tc.reconciliationStatus,
				Kubeconfig:           test.ReadKubeconfig(t),
				ComponentCRDs:        tc.componentCRDs,
			})
			for idx, expected := range tc.expected.Queue {
				require.ElementsMatch(t, result.Queue[idx], expected)
			}
		})
	}
}
