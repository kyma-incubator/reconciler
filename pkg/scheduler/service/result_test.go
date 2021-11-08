package service

import (
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	operations      []*model.OperationEntity
	expectedResult  model.Status
	expectedOrphans []string //contains correlation IDs
}

func TestReconciliationResult(t *testing.T) {
	testCases := []*testCase{
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.1",
					State:         model.OperationStateNew,
					Updated:       time.Now().Add(-1999 * time.Millisecond),
				},
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.2",
					State:         model.OperationStateError,
					Updated:       time.Now().Add(-2000 * time.Millisecond),
				},
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.3",
					State:         model.OperationStateInProgress,
					Updated:       time.Now().Add(-2001 * time.Millisecond),
				},
			},
			expectedResult:  model.ClusterStatusReconcileError,
			expectedOrphans: []string{"1.3"},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.1",
					State:         model.OperationStateFailed,
					Updated:       time.Now().Add(-3 * time.Second),
				},
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.2",
					State:         model.OperationStateNew,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.3",
					State:         model.OperationStateInProgress,
					Updated:       time.Now(),
				},
			},
			expectedResult:  model.ClusterStatusReconciling,
			expectedOrphans: []string{"1.1"},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.1",
					State:         model.OperationStateDone,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.2",
					State:         model.OperationStateNew,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.3",
					State:         model.OperationStateInProgress,
					Updated:       time.Now(),
				},
			},
			expectedResult: model.ClusterStatusReconciling,
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.1",
					State:         model.OperationStateDone,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "2.1",
					State:         model.OperationStateNew,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.2",
					State:         model.OperationStateInProgress,
					Updated:       time.Now(),
				},
			},
			expectedResult: model.ClusterStatusReconciling,
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.1",
					State:         model.OperationStateDone,
				},
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.2",
					State:         model.OperationStateDone,
				},
			},
			expectedResult: model.ClusterStatusReady,
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.1",
					State:         model.OperationStateError,
				},
				{
					Priority:      1,
					SchedulingID:  "schedulingID",
					CorrelationID: "1.2",
					State:         model.OperationStateDone,
				},
			},
			expectedResult: model.ClusterStatusReconcileError,
		},
	}

	for _, testCase := range testCases {
		reconResult := newReconciliationResult(&model.ReconciliationEntity{
			RuntimeID:    "runtimeID",
			SchedulingID: "schedulingID",
		}, 2*time.Second, logger.NewLogger(true))

		require.NoError(t, reconResult.AddOperations(testCase.operations))

		require.Equal(t, reconResult.GetResult(), testCase.expectedResult)

		//check detected orphans
		allDetectedOrphans := make(map[string]*model.OperationEntity)
		detectedOrphans := reconResult.GetOrphans()
		for _, detectedOrphan := range detectedOrphans {
			allDetectedOrphans[detectedOrphan.CorrelationID] = detectedOrphan
		}
		for _, correlationID := range testCase.expectedOrphans {
			_, ok := allDetectedOrphans[correlationID]
			require.True(t, ok)
		}
	}
}
