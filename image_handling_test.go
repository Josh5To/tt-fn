package main

import (
	_ "embed"
	"testing"
)

//go:embed test_files/b64_test_image
var b64DataFromFile string

func Test_base64toPNG(t *testing.T) {
	type args struct {
		b64Data  string
		filepath string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "write file to png",
			args:    args{b64Data: b64DataFromFile, filepath: "./test_files/testerpng1.png"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := base64toPNG(tt.args.b64Data, tt.args.filepath); (err != nil) != tt.wantErr {
				t.Errorf("base64toPNG() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
