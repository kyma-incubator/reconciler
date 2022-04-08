package reconciliation

import (
	"reflect"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	interfaceSliceZeroValue = []interface{}{}
)

func (s *reconciliationTestSuite) Test_toInterfaceSlice() {
	t := s.T()
	type args struct {
		args []string
	}
	tests := []struct {
		name string
		args args
		want []interface{}
	}{
		{
			name: "nil",
			args: args{},
			want: interfaceSliceZeroValue,
		},
		{
			name: "empty",
			args: args{
				args: []string{},
			},
			want: interfaceSliceZeroValue,
		},
		{
			name: "some",
			args: args{
				args: []string{"test", "me", "plz"},
			},
			want: []interface{}{"test", "me", "plz"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := toInterfaceSlice(tt.args.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toInterfaceSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (s *reconciliationTestSuite) TestFilterMixer_FilterByQuery() {
	t := s.T()
	testLogger := zap.NewExample().Sugar()
	defer func() {
		if err := testLogger.Sync(); err != nil {
			t.Logf("while flushing logs: %s", err)
		}
	}()
	now := time.Now()

	tests := []struct {
		name      string
		filters   []Filter
		wantErr   bool
		wantQuery string
	}{
		{
			name:      "ok with no filters",
			wantErr:   false,
			wantQuery: "",
		},
		{
			name: "ok with limit filter",
			filters: []Filter{
				&Limit{Count: 12},
			},
			wantErr:   false,
			wantQuery: " ORDER BY created DESC LIMIT 12",
		},
		{
			name: "ok with a few filters",
			filters: []Filter{
				&WithRuntimeIDs{RuntimeIDs: []string{"test-1", "test-2"}},
				&WithCreationDateAfter{Time: now.Add(-100 * time.Second)},
				&WithCreationDateBefore{Time: now},
				&WithStatuses{Statuses: []string{"test-status-1", "test-status-2"}},
			},
			wantErr:   false,
			wantQuery: " WHERE runtime_id IN ($1,$2) AND (created>$3) AND (created<$4) AND (status=$5 OR status=$6)",
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			q, err := db.NewQuery(&db.MockConnection{}, &model.ReconciliationEntity{}, testLogger)
			require.NoError(t, err)
			s := &db.Select{
				Query: q,
			}
			fm := &FilterMixer{
				Filters: tt.filters,
			}

			err = fm.FilterByQuery(s)

			require.NoError(t, err)
			require.Equal(t, s.String(), tt.wantQuery)
		})
	}
}

func (s *reconciliationTestSuite) TestFilterMixer_FilterByInstance() {
	t := s.T()
	tests := []struct {
		name    string
		filters []Filter
		give    *model.ReconciliationEntity
		want    *model.ReconciliationEntity
	}{
		{
			name: "no filters",
			give: &model.ReconciliationEntity{},
			want: &model.ReconciliationEntity{},
		},
		{
			name: "pass with a few flters",
			filters: []Filter{
				&WithRuntimeID{RuntimeID: "test-id"},
				&WithStatuses{Statuses: []string{"created", "error"}},
			},
			give: &model.ReconciliationEntity{
				RuntimeID: "test-id",
				Finished:  false,
				Status:    "error",
			},
			want: &model.ReconciliationEntity{
				RuntimeID: "test-id",
				Finished:  false,
				Status:    "error",
			},
		},
		{
			name: "return nil",
			filters: []Filter{
				&WithRuntimeID{RuntimeID: "test-id"},
				&WithStatuses{Statuses: []string{"created", "error"}},
			},
			give: &model.ReconciliationEntity{
				RuntimeID: "wrong-test-id",
				Finished:  false,
				Status:    "error",
			},
			want: nil,
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			fm := &FilterMixer{
				Filters: tt.filters,
			}

			got := fm.FilterByInstance(tt.give)

			require.Equal(t, tt.want, got)
		})
	}
}

func (s *reconciliationTestSuite) Test_columnName() {
	t := s.T()
	testLogger := zap.NewExample().Sugar()
	defer func() {
		if err := testLogger.Sync(); err != nil {
			t.Logf("while flushing logs: %s", err)
		}
	}()

	t.Run("get name", func(t *testing.T) {
		q, err := db.NewQuery(&db.MockConnection{}, &model.ReconciliationEntity{}, testLogger)
		require.NoError(t, err)
		s := &db.Select{
			Query: q,
		}
		got, err := columnName(s, "RuntimeID")
		require.NoError(t, err)
		require.Equal(t, "runtime_id", got)
	})

	t.Run("error - column doesn't exist", func(t *testing.T) {
		q, err := db.NewQuery(&db.MockConnection{}, &model.ReconciliationEntity{}, testLogger)
		require.NoError(t, err)
		s := &db.Select{
			Query: q,
		}
		got, err := columnName(s, "RuntimeIDIDIDID")
		require.Error(t, err)
		require.Equal(t, "", got)
	})
}
