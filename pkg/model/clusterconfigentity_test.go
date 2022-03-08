package model

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	entity1 *ClusterConfigurationEntity
	entity2 *ClusterConfigurationEntity
	equal   bool
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
