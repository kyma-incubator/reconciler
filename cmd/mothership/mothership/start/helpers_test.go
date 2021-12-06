package cmd

import (
	"reflect"
	"testing"

	"github.com/kyma-incubator/reconciler/pkg/keb"
	"github.com/kyma-incubator/reconciler/pkg/model"
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
