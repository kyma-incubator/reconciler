package reconciliation

import (
	"reflect"
	"testing"
)

var (
	interfaceSliceZeroValue = []interface{}{}
)

func Test_toInterfaceSlice(t *testing.T) {
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
