package service

import (
	"github.com/kyma-incubator/reconciler/pkg/logger"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testCase struct {
	operations      []*model.OperationEntity
	expectedResults map[string]model.Status
	expectedOrphans []string //contains correlation IDs
}

func TestBookkeeper_processClusterStateAndOrphans(t *testing.T) {
	bk := newBookkeeper(nil, &BookkeeperConfig{
		OrphanOperationTimeout: 2 * time.Second,
	}, logger.NewLogger(true))

	testCases := []*testCase{
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateNew,
					Updated:       time.Now().Add(-1999 * time.Millisecond),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.2",
					State:         model.OperationStateError,
					Updated:       time.Now().Add(-2000 * time.Millisecond),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.3",
					State:         model.OperationStateInProgress,
					Updated:       time.Now().Add(-2001 * time.Millisecond),
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusError,
			},
			expectedOrphans: []string{"1.3"},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateFailed,
					Updated:       time.Now().Add(-3 * time.Second),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.2",
					State:         model.OperationStateNew,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.3",
					State:         model.OperationStateInProgress,
					Updated:       time.Now(),
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusReconciling,
			},
			expectedOrphans: []string{"1.1"},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateDone,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.2",
					State:         model.OperationStateNew,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.3",
					State:         model.OperationStateInProgress,
					Updated:       time.Now(),
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusReconciling,
			},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateDone,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "2",
					CorrelationID: "2.1",
					State:         model.OperationStateNew,
					Updated:       time.Now(),
				},
				{
					Priority:      1,
					SchedulingID:  "2",
					CorrelationID: "2.2",
					State:         model.OperationStateInProgress,
					Updated:       time.Now(),
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusReady,
				"2": model.ClusterStatusReconciling,
			},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateDone,
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.2",
					State:         model.OperationStateDone,
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusReady,
			},
		},
		{
			operations: []*model.OperationEntity{
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.1",
					State:         model.OperationStateError,
				},
				{
					Priority:      1,
					SchedulingID:  "1",
					CorrelationID: "1.2",
					State:         model.OperationStateDone,
				},
			},
			expectedResults: map[string]model.Status{
				"1": model.ClusterStatusError,
			},
		},
	}
	for _, testCase := range testCases {
		reconResults, err := bk.processReconciliations(testCase.operations)
		require.NoError(t, err)
		require.Equal(t, len(testCase.expectedResults), len(reconResults))

		//check calculated cluster results
		for schedulingID, expectedStatus := range testCase.expectedResults {
			reconResult, ok := reconResults[schedulingID]
			require.True(t, ok)
			require.Equal(t, expectedStatus, reconResult.GetResult())
		}

		//check detected orphans
		allDetectedOrphans := make(map[string]*model.OperationEntity)
		for _, reconResult := range reconResults {
			detectedOrphans := reconResult.GetOrphans()
			for _, detectedOrphan := range detectedOrphans {
				allDetectedOrphans[detectedOrphan.CorrelationID] = detectedOrphan
			}
		}
		for _, correlationID := range testCase.expectedOrphans {
			_, ok := allDetectedOrphans[correlationID]
			require.True(t, ok)
		}
	}
}
