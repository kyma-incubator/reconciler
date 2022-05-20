package repository

import (
	"math"
	"reflect"
	"testing"
)

func Test_SplitSliceByBlockSize(t *testing.T) {
	type args struct {
		slice     []interface{}
		blockSize int
	}
	tests := []struct {
		name string
		args args
		want [][]interface{}
	}{
		{
			name: "when block size is more than max int32",
			args: args{
				slice:     []interface{}{"item1", "item2", "item3"},
				blockSize: math.MaxInt32,
			},
			want: [][]interface{}{{"item1", "item2", "item3"}},
		},
		{
			name: "when a slice of 9 items should be split into blocks of 3",
			args: args{
				slice:     []interface{}{"item1", "item2", "item3", "item4", "item5", "item6", "item7", "item8", "item9"},
				blockSize: 3,
			},
			want: [][]interface{}{{"item1", "item2", "item3"}, {"item4", "item5", "item6"}, {"item7", "item8", "item9"}},
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			got := SplitSliceByBlockSize(testCase.args.slice, testCase.args.blockSize)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("splitStringSlice() got = %v, want %v", got, testCase.want)
			}
		})
	}
}
