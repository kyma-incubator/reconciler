package reconciliation

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func (s *reconciliationTestSuite) TestPersistentReconciliationRepository_RemoveReconciliationsBeforeDeadline() {
	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with no reconciliations",
			wantErr:         false,
			reconciliations: 0,
		},
		{
			name:            "with one reconciliation",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with multiple reconciliations",
			wantErr:         false,
			reconciliations: 101,
		},
	}
	for _, tt := range tests {
		t := s.T()
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testEntities := s.prepareTest(t, testCase.reconciliations)
			timeTo := time.Now().UTC()
			for _, runtimeID := range s.runtimeIDs {
				if err := s.persistenceRepo.RemoveReconciliationsBeforeDeadline(runtimeID, "nonExistentToMockDeletion", timeTo); (err != nil) != testCase.wantErr {
					t.Errorf("Persistence RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
				}
				if err := testEntities.inMemoryRepo.RemoveReconciliationsBeforeDeadline(runtimeID, "nonExistentToMockDeletion", timeTo); (err != nil) != testCase.wantErr {
					t.Errorf("InMemory RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
				}
			}

			// check - also ensures clean up
			persistenceReconciliations, err := s.persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := testEntities.inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
		})
	}
}

func (s *reconciliationTestSuite) TestPersistentReconciliationRepository_GetRuntimeIDs() {
	t := s.T()
	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with no reconciliations",
			wantErr:         false,
			reconciliations: 0,
		},
		{
			name:            "with one reconciliation",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with multiple reconciliations",
			wantErr:         false,
			reconciliations: 11,
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testEntities := s.prepareTest(t, testCase.reconciliations)
			persistentRepoRuntimeIDs, err := s.persistenceRepo.GetRuntimeIDs()
			require.NoError(t, err)
			inmemoryRepoRuntimeIDs, err := testEntities.inMemoryRepo.GetRuntimeIDs()
			require.NoError(t, err)
			sort.Strings(s.runtimeIDs)
			sort.Strings(persistentRepoRuntimeIDs)
			sort.Strings(inmemoryRepoRuntimeIDs)
			require.True(t, reflect.DeepEqual(s.runtimeIDs, persistentRepoRuntimeIDs))
			require.NoError(t, err)
			require.True(t, reflect.DeepEqual(s.runtimeIDs, inmemoryRepoRuntimeIDs))
			s.AfterTest("", testCase.name)
		})
	}
}

func (s *reconciliationTestSuite) TestPersistentReconciliationRepository_RemoveReconciliationsBySchedulingID() {
	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with no reconciliations",
			wantErr:         false,
			reconciliations: 0,
		},
		{
			name:            "with one reconciliation",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with multiple reconciliations less than 200 (1 block)",
			wantErr:         false,
			reconciliations: 69,
		},
		{
			name:            "with multiple reconciliations more than 200 (3 blocks)",
			wantErr:         false,
			reconciliations: 409,
		},
	}
	for _, tt := range tests {
		t := s.T()
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testEntities := s.prepareTest(t, testCase.reconciliations)

			if err := s.persistenceRepo.RemoveReconciliationsBySchedulingID(testEntities.persistenceSchedulingIDs); (err != nil) != testCase.wantErr {
				t.Errorf("Persistence RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if err := testEntities.inMemoryRepo.RemoveReconciliationsBySchedulingID(testEntities.inMemorySchedulingIDs); (err != nil) != testCase.wantErr {
				t.Errorf("InMemory RemoveSchedulingIds() error = %v, wantErr %v", err, testCase.wantErr)
			}

			// check - also ensures clean up
			persistenceReconciliations, err := s.persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := testEntities.inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
			s.AfterTest("", testCase.name)
		})
	}
}

func (s *reconciliationTestSuite) TestPersistentReconciliationRepository_RemoveReconciliationByRuntimeID() {
	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with one runtime ID",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with no runtime ID",
			wantErr:         false,
			reconciliations: 0,
		},
	}
	for _, tt := range tests {
		t := s.T()
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testEntities := s.prepareTest(t, testCase.reconciliations)
			var runtimeID string
			if testCase.reconciliations > 0 {
				runtimeID = s.runtimeIDs[0]
			}
			if err := s.persistenceRepo.RemoveReconciliationByRuntimeID(runtimeID); (err != nil) != testCase.wantErr {
				t.Errorf("Persistence RemoveReconciliationByRuntimeID() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if err := testEntities.inMemoryRepo.RemoveReconciliationByRuntimeID(runtimeID); (err != nil) != testCase.wantErr {
				t.Errorf("InMemory RemoveReconciliationByRuntimeID() error = %v, wantErr %v", err, testCase.wantErr)
			}

			// check - also ensures clean up
			persistenceReconciliations, err := s.persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := testEntities.inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
			s.AfterTest("", testCase.name)
		})
	}
}

func (s *reconciliationTestSuite) TestPersistentReconciliationRepository_RemoveReconciliationBySchedulingID() {
	tests := []struct {
		name            string
		wantErr         bool
		reconciliations int
	}{
		{
			name:            "with one reconciliation",
			wantErr:         false,
			reconciliations: 1,
		},
		{
			name:            "with no reconciliations",
			wantErr:         false,
			reconciliations: 0,
		},
	}
	for _, tt := range tests {
		t := s.T()
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			testEntities := s.prepareTest(t, testCase.reconciliations)
			var schedulingIDPersistent, schedulingIDInMemory string
			if testCase.reconciliations > 0 {
				schedulingIDPersistent = testEntities.persistenceSchedulingIDs[0].(string)
				schedulingIDInMemory = testEntities.inMemorySchedulingIDs[0].(string)
			}
			if err := s.persistenceRepo.RemoveReconciliationBySchedulingID(schedulingIDPersistent); (err != nil) != testCase.wantErr {
				t.Errorf("Persistence RemoveReconciliationBySchedulingID() error = %v, wantErr %v", err, testCase.wantErr)
			}
			if err := testEntities.inMemoryRepo.RemoveReconciliationBySchedulingID(schedulingIDInMemory); (err != nil) != testCase.wantErr {
				t.Errorf("InMemory RemoveReconciliationBySchedulingID() error = %v, wantErr %v", err, testCase.wantErr)
			}

			// check - also ensures clean up
			persistenceReconciliations, err := s.persistenceRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(persistenceReconciliations))
			inMemoryReconciliations, err := testEntities.inMemoryRepo.GetReconciliations(&WithCreationDateBefore{Time: time.Now()})
			require.NoError(t, err)
			require.Equal(t, 0, len(inMemoryReconciliations))
			s.AfterTest("", testCase.name)
		})
	}
}
