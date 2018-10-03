package rethink

import (
	"context"
	"reflect"
	"testing"

	"github.com/ramrod-project/backend-controller-go/customtypes"
)

func Test_logSend(t *testing.T) {

	tests := []struct {
		name    string
		log     customtypes.ContainerLog
		wantErr bool
	}{
		{
			name: "test log 1",
			log: customtypes.ContainerLog{
				ContainerID:   "284813vm8y13-13v8y9-713yv1",
				ContainerName: "some-service-name.0whatever",
				Log:           "[INFO] blahblahblahblhbq 39 4g0wo 43589pqhwpr8g4",
				ServiceName:   "some-service-name",
			},
			wantErr: false,
		},
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
