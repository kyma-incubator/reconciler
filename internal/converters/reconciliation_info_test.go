package converters_test

import (
	"github.com/kyma-incubator/reconciler/internal/converters"
	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConvertReconciliationStatus(t *testing.T) {
	//GIVEN
	reconEntInput := &model.ReconciliationEntity{
		Lock:                "lock",
		RuntimeID:           "runtime",
		ClusterConfig:       2,
		ClusterConfigStatus: -2,
		Finished:            true,
		SchedulingID:        "1234",
		Created:             time.Unix(0, 10),
		Updated:             time.Unix(10, 100),
		Status:              model.ClusterStatusReconcileDisabled,
	}

	opEntInput := &model.OperationEntity{
		Priority:      2,
		SchedulingID:  "abcd",
		CorrelationID: "zxcv",
		RuntimeID:     "runtime",
		ClusterConfig: 5,
		Component:     "testComponent",
		Type:          model.OperationTypeDelete,
		State:         "testState",
		Reason:        "unit test",
		Created:       time.Unix(0, 8),
		Updated:       time.Unix(80, 800),
	}
	testCases := map[string]struct {
		opEntInput []*model.OperationEntity
	}{
		"empty operation array": {
			opEntInput: []*model.OperationEntity{},
		},
		"not empty operation array": {
			opEntInput: []*model.OperationEntity{opEntInput, opEntInput},
		},
	}

	for name, testCase := range testCases {
		tc := testCase
		t.Run(name, func(t *testing.T) {
			//WHEN
			output, err := converters.ConvertReconciliation(reconEntInput, tc.opEntInput)

			//THEN
			require.NoError(t, err)
			assertReconciliation(t, reconEntInput, output)

			outLen := len(output.Operations)
			inLen := len(tc.opEntInput)
			require.Equal(t, inLen, outLen)

			for i, outOp := range output.Operations {
				assertOperation(t, tc.opEntInput[i], outOp)
			}
		})
	}

}

func assertReconciliation(t *testing.T, input *model.ReconciliationEntity, output keb.ReconciliationInfoOKResponse) {
	assert.Equal(t, input.RuntimeID, output.RuntimeID)
	assert.Equal(t, input.ClusterConfig, output.ConfigVersion)
	assert.Equal(t, input.Finished, output.Finished)
	assert.Equal(t, input.SchedulingID, output.SchedulingID)
	assert.Equal(t, input.Created, output.Created)
	assert.Equal(t, input.Updated, output.Updated)
}

func assertOperation(t *testing.T, input *model.OperationEntity, output keb.Operation) {
	assert.Equal(t, input.Component, output.Component)
	assert.Equal(t, input.CorrelationID, output.CorrelationID)
	assert.Equal(t, input.Created, output.Created)
	assert.Equal(t, input.Priority, output.Priority)
	assert.Equal(t, input.Reason, output.Reason)
	assert.Equal(t, input.SchedulingID, output.SchedulingID)
	assert.Equal(t, string(input.State), output.State)
	assert.Equal(t, input.Updated, output.Updated)
}
