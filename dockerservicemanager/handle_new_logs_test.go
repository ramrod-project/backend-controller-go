package dockerservicemanager

import (
	"context"
	"reflect"
	"testing"

	"github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/customtypes"
)

func Test_newContainerLogger(t *testing.T) {
	type args struct {
		ctx          context.Context
		dockerClient *client.Client
		name         string
	}
	tests := []struct {
		name  string
		args  args
		want  <-chan customtypes.ContainerLog
		want1 <-chan error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := newContainerLogger(tt.args.ctx, tt.args.dockerClient, tt.args.name)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newContainerLogger() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("newContainerLogger() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestNewLogHandler(t *testing.T) {
	type args struct {
		ctx      context.Context
		newNames <-chan string
	}
	tests := []struct {
		name  string
		args  args
		want  <-chan (<-chan customtypes.ContainerLog)
		want1 <-chan error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := NewLogHandler(tt.args.ctx, tt.args.newNames)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewLogHandler() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("NewLogHandler() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
