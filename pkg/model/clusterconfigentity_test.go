package model

import (
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/stretchr/testify/require"
	"testing"
)

type testCase struct {
	entity1 *ClusterConfigurationEntity
	entity2 *ClusterConfigurationEntity
	equal   bool
}

func TestClusterConfigEntity(t *testing.T) {
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
