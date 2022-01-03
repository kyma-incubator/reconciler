package operation

import (
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/db"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFilterMixer_FilterByQuery(t *testing.T) {
	testLogger := zap.NewExample().Sugar()
	defer func() {
		if err := testLogger.Sync(); err != nil {
			t.Logf("while flushing logs: %s", err)
		}
	}()

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
			name: "ok with schedulingID filter",
			filters: []Filter{
				&WithSchedulingID{SchedulingID: "test-scheduling-id"},
			},
			wantErr:   false,
			wantQuery: " WHERE scheduling_id=$1",
		},
		{
			name: "ok with a few filters",
			filters: []Filter{
				&WithSchedulingID{SchedulingID: "test-scheduling-id"},
				&WithStates{States: []model.OperationState{"state1", "state2"}},
			},
			wantErr:   false,
			wantQuery: " WHERE scheduling_id=$1 AND state IN ($2,$3)",
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			q, err := db.NewQuery(&db.MockConnection{}, &model.OperationEntity{}, testLogger)
			require.NoError(t, err)
			s := &db.Select{
				Query: q,
			}
			fm := &FilterMixer{
				Filters: tt.filters,
			}

			err = fm.FilterByQuery(s)

			require.NoError(t, err)
			require.Equal(t, tt.wantQuery, s.String())
		})
	}
}
