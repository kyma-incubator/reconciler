package cmd

import (
	"reflect"
	"testing"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
	"github.com/stretchr/testify/require"
)

func Test_components(t *testing.T) {
	type args struct {
		cfg model.ClusterConfigurationEntity
	}
	tests := []struct {
		name string
		args args
		want []keb.Component
	}{
		{
			name: "nil",
			args: args{},
			want: []keb.Component{},
		},
		{
			name: "empty",
			args: args{
				cfg: model.ClusterConfigurationEntity{},
			},
			want: []keb.Component{},
		},
		{
			name: "some",
			args: args{
				cfg: model.ClusterConfigurationEntity{
					Components: []*keb.Component{
						{
							Component: "a1",
							URL:       "a1",
							Version:   "a1",
						},
					},
				},
			},
			want: []keb.Component{
				{
					URL:       "a1",
					Component: "a1",
					Version:   "a1",
				},
			},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if got := components(tt.args.cfg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("components() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_filterReconciliationsAfter(t *testing.T) {
	tNow := time.Now()

	type args struct {
		time            time.Time
		reconciliations []keb.Reconciliation
	}
	tests := []struct {
		name string
		args args
		want []keb.Reconciliation
	}{
		{
			name: "return empty list",
			want: []keb.Reconciliation{},
		},
		{
			name: "return all items",
			args: args{
				time: tNow.Add(-10 * time.Hour),
				reconciliations: []keb.Reconciliation{
					{
						Created: tNow,
					},
					{
						Created: tNow.Add(-1 * time.Hour),
					},
					{
						Created: tNow.Add(-2 * time.Hour),
					},
				},
			},
			want: []keb.Reconciliation{
				{
					Created: tNow,
				},
				{
					Created: tNow.Add(-1 * time.Hour),
				},
				{
					Created: tNow.Add(-2 * time.Hour),
				},
			},
		},
		{
			name: "return only a few items",
			args: args{
				time: tNow.Add(-10 * time.Hour),
				reconciliations: []keb.Reconciliation{
					{
						Created: tNow.Add(-15 * time.Hour),
					},
					{
						Created: tNow.Add(-20 * time.Hour),
					},
					{
						Created: tNow,
					},
					{
						Created: tNow.Add(-1 * time.Hour),
					},
					{
						Created: tNow.Add(-2 * time.Hour),
					},
				},
			},
			want: []keb.Reconciliation{
				{
					Created: tNow,
				},
				{
					Created: tNow.Add(-1 * time.Hour),
				},
				{
					Created: tNow.Add(-2 * time.Hour),
				},
			},
		},
		{
			name: "return zero items",
			args: args{
				time: tNow.Add(-10 * time.Hour),
				reconciliations: []keb.Reconciliation{
					{
						Created: tNow.Add(-15 * time.Hour),
					},
					{
						Created: tNow.Add(-20 * time.Hour),
					},
					{
						Created: tNow.Add(-11 * time.Hour),
					},
					{
						Created: tNow.Add(-22 * time.Hour),
					},
				},
			},
			want: []keb.Reconciliation{},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			got := filterReconciliationsAfter(tt.args.time, tt.args.reconciliations)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_filterReconciliationsBefore(t *testing.T) {
	tNow := time.Now()

	type args struct {
		time            time.Time
		reconciliations []keb.Reconciliation
	}
	tests := []struct {
		name string
		args args
		want []keb.Reconciliation
	}{
		{
			name: "return empty list",
			want: []keb.Reconciliation{},
		},
		{
			name: "return all items",
			args: args{
				time: tNow.Add(-10 * time.Hour),
				reconciliations: []keb.Reconciliation{
					{
						Created: tNow.Add(-11 * time.Hour),
					},
					{
						Created: tNow.Add(-21 * time.Hour),
					},
				},
			},
			want: []keb.Reconciliation{
				{
					Created: tNow.Add(-11 * time.Hour),
				},
				{
					Created: tNow.Add(-21 * time.Hour),
				},
			},
		},
		{
			name: "return only a few items",
			args: args{
				time: tNow.Add(-10 * time.Hour),
				reconciliations: []keb.Reconciliation{
					{
						Created: tNow,
					},
					{
						Created: tNow.Add(-15 * time.Hour),
					},
					{
						Created: tNow.Add(-20 * time.Hour),
					},
					{
						Created: tNow.Add(-1 * time.Hour),
					},
					{
						Created: tNow.Add(-2 * time.Hour),
					},
				},
			},
			want: []keb.Reconciliation{
				{
					Created: tNow.Add(-15 * time.Hour),
				},
				{
					Created: tNow.Add(-20 * time.Hour),
				},
			},
		},
		{
			name: "return zero items",
			args: args{
				time: tNow.Add(-10 * time.Hour),
				reconciliations: []keb.Reconciliation{
					{
						Created: tNow.Add(-1 * time.Hour),
					},
					{
						Created: tNow.Add(-2 * time.Hour),
					},
					{
						Created: tNow.Add(-3 * time.Hour),
					},
					{
						Created: tNow.Add(-4 * time.Hour),
					},
				},
			},
			want: []keb.Reconciliation{},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			got := filterReconciliationsBefore(tt.args.time, tt.args.reconciliations)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_filterReconciliationsTail(t *testing.T) {
	tNow := time.Now()

	type args struct {
		l               int
		reconciliations []keb.Reconciliation
	}
	tests := []struct {
		name string
		args args
		want []keb.Reconciliation
	}{
		{
			name: "return empty list",
			args: args{
				reconciliations: []keb.Reconciliation{},
				l:               0,
			},
			want: []keb.Reconciliation{},
		},
		{
			name: "return empty list with l=0",
			args: args{
				reconciliations: []keb.Reconciliation{
					{},
					{},
					{},
					{},
				},
				l: 0,
			},
			want: []keb.Reconciliation{},
		},
		{
			name: "return a few elements",
			args: args{
				reconciliations: []keb.Reconciliation{
					{
						Created: tNow.Add(-3 * time.Hour),
					},
					{
						Created: tNow.Add(-4 * time.Hour),
					},
					{
						Created: tNow.Add(-6 * time.Hour),
					},
					{
						Created: tNow.Add(-7 * time.Hour),
					},
					{
						Created: tNow,
					},
					{
						Created: tNow.Add(-5 * time.Hour),
					},
					{
						Created: tNow.Add(-1 * time.Hour),
					},
					{
						Created: tNow.Add(-2 * time.Hour),
					},
				},
				l: 5,
			},
			want: []keb.Reconciliation{
				{
					Created: tNow,
				},
				{
					Created: tNow.Add(-1 * time.Hour),
				},
				{
					Created: tNow.Add(-2 * time.Hour),
				},
				{
					Created: tNow.Add(-3 * time.Hour),
				},
				{
					Created: tNow.Add(-4 * time.Hour),
				},
			},
		},
		{
			name: "return whole list with l > len(list)",
			args: args{
				reconciliations: []keb.Reconciliation{
					{
						Created: tNow,
					},
					{
						Created: tNow.Add(-1 * time.Hour),
					},
					{
						Created: tNow.Add(-2 * time.Hour),
					},
					{
						Created: tNow.Add(-3 * time.Hour),
					},
					{
						Created: tNow.Add(-4 * time.Hour),
					},
				},
				l: 50,
			},
			want: []keb.Reconciliation{
				{
					Created: tNow,
				},
				{
					Created: tNow.Add(-1 * time.Hour),
				},
				{
					Created: tNow.Add(-2 * time.Hour),
				},
				{
					Created: tNow.Add(-3 * time.Hour),
				},
				{
					Created: tNow.Add(-4 * time.Hour),
				},
			},
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			got := filterReconciliationsTail(tt.args.reconciliations, tt.args.l)
			require.Equal(t, tt.want, got)
		})
	}
}
