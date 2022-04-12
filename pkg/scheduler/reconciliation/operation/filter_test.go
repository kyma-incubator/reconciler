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
				&WithCorrelationID{CorrelationID: "test-correlation-id"},
				&WithStates{States: []model.OperationState{"state1", "state2"}},
				&WithComponentName{Component: "component1"},
				&Limit{Count: 1},
			},
			wantErr:   false,
			wantQuery: " WHERE scheduling_id=$1 AND correlation_id=$2 AND state IN ($3,$4) AND component=$5 ORDER BY created DESC LIMIT 1",
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			q, err := db.NewQuery(&db.MockConnection{}, &model.OperationEntity{}, testLogger)
			require.NoError(t, err)
			s := &db.Select{
				QueryOld: q,
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
