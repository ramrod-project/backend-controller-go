package rethink

import (
	"context"
	"reflect"
	"testing"

	"github.com/ramrod-project/backend-controller-go/customtypes"
	r "gopkg.in/gorethink/gorethink.v4"
)

func Test_logSend(t *testing.T) {
	type args struct {
		sess *r.Session
		log  customtypes.ContainerLog
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := logSend(tt.args.sess, tt.args.log); (err != nil) != tt.wantErr {
				t.Errorf("logSend() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAggregateLogs(t *testing.T) {
	type args struct {
		ctx      context.Context
		logChans <-chan (<-chan customtypes.ContainerLog)
	}
	tests := []struct {
		name string
		args args
		want <-chan error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AggregateLogs(tt.args.ctx, tt.args.logChans); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AggregateLogs() = %v, want %v", got, tt.want)
			}
		})
	}
}
