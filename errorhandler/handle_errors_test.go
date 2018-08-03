package errorhandler

import "testing"

func TestErrorHandler(t *testing.T) {
	type args struct {
		errorChans []<-chan error
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ErrorHandler(tt.args.errorChans...)
		})
	}
}
