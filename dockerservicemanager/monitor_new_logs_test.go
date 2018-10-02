package dockerservicemanager

import (
	"context"
	"reflect"
	"testing"

	client "github.com/docker/docker/client"
)

func Test_stackContainerIDs(t *testing.T) {
	type args struct {
		ctx          context.Context
		dockerClient *client.Client
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stackContainerIDs(tt.args.ctx, tt.args.dockerClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("stackContainerIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("stackContainerIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}
