package reconciliation

import (
	"fmt"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"math"
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
				schedulingIDPersistent = testEntities.persistenceSchedulingIDs[0]
				schedulingIDInMemory = testEntities.inMemorySchedulingIDs[0]
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

func (s *reconciliationTestSuite) TestPersistentReconciliationRepository_RemoveReconciliationsForObsoleteStatus() {
	tests := []struct {
		name     string
		count    int
		want     int
		wantErr  bool
		obsolete bool
		deleted  bool
	}{
		{
			name:     "when reconciliations refer to obsolete cluster config statuses",
			want:     5,
			count:    5,
			deleted:  true,
			obsolete: true,
		},
		{
			name:     "when reconciliations refer to older non-deleted cluster config statuses",
			want:     0,
			count:    5,
			obsolete: true,
			deleted:  false,
		},
		{
			name:     "when reconciliations refer to current deleted cluster config statuses",
			want:     0,
			count:    5,
			obsolete: false,
			deleted:  true,
		},
		{
			name:  "when no reconciliations are present",
			want:  0,
			count: 0,
		},
	}
	for _, tt := range tests {
		t := s.T()
		testCase := tt
		t.Run(tt.name, func(t *testing.T) {
			s.prepareTest(t, testCase.count)
			statusEntity := &model.ClusterStatusEntity{}
			clusterEntity := &model.ClusterEntity{}
			dbConn := s.TxConnection()

			if testCase.deleted {
				for _, runtimeID := range s.runtimeIDs {
					updateSQLTemplate := "UPDATE %s SET %s=$1 WHERE %s=$2"
					_, err := dbConn.Exec(fmt.Sprintf(updateSQLTemplate, statusEntity.Table(), "deleted", "runtime_id"), "TRUE", runtimeID)
					require.NoError(t, err)
					_, err = dbConn.Exec(fmt.Sprintf(updateSQLTemplate, clusterEntity.Table(), "deleted", "runtime_id"), "TRUE", runtimeID)
					require.NoError(t, err)
				}
			}

			deadline := time.Now().UTC()
			if !testCase.obsolete {
				deadline = deadline.Add(-time.Hour)
			}
			got, err := s.persistenceRepo.RemoveReconciliationsForObsoleteStatus(deadline)
			if (err != nil) != testCase.wantErr {
				t.Errorf("RemoveReconciliationsForObsoleteStatus() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if got != testCase.want {
				t.Errorf("RemoveReconciliationsForObsoleteStatus() got = %v, want %v", got, testCase.want)
			}
			s.AfterTest("", testCase.name)
		})
	}
}

func (s *reconciliationTestSuite) Test_splitStringSlice() {
	type args struct {
		slice     []string
		blockSize int
	}
	tests := []struct {
		name string
		args args
		want [][]string
	}{
		{
			name: "when block size is more than max int32",
			args: args{
				slice:     []string{"item1", "item2", "item3"},
				blockSize: math.MaxInt32,
			},
			want: [][]string{{"item1", "item2", "item3"}},
		},
		{
			name: "when a slice of 9 items should be split into blocks of 3",
			args: args{
				slice:     []string{"item1", "item2", "item3", "item4", "item5", "item6", "item7", "item8", "item9"},
				blockSize: 3,
			},
			want: [][]string{{"item1", "item2", "item3"}, {"item4", "item5", "item6"}, {"item7", "item8", "item9"}},
		},
	}
	for _, tt := range tests {
		t := s.T()
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			got := splitStringSlice(testCase.args.slice, testCase.args.blockSize)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("splitStringSlice() got = %v, want %v", got, testCase.want)
			}
		})
	}
}
