package tcp

import (
	"reflect"
	"testing"
)

func Test_convert(t *testing.T) {
	type args struct {
		display uint8
		data    []byte
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{
				display: 2,
				data:    []byte{0x59, 0xdf, 0x3d, 0x1a},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convert(tt.args.display, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("convert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convert() got = %v, want %v", got, tt.want)
			}
		})
	}
}
