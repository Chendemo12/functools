package logger

import "testing"

func TestDebug(t *testing.T) {
	type args struct {
		args []any
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "1",
			args: args{
				args: []any{"111"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Debug(tt.args.args...)
		})
	}
}

func TestErrorf(t *testing.T) {
	type args struct {
		format string
		v      []any
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "errorf",
			args: args{
				format: "this is '%d'",
				v:      []any{12},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Errorf(tt.args.format, tt.args.v...)
		})
	}
}
