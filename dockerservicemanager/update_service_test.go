package dockerservicemanager

import (
	"reflect"
	"testing"

	"github.com/docker/docker/api/types"
)

func TestUpdatePluginService(t *testing.T) {
	type args struct {
		serviceID string
		config    *PluginServiceConfig
	}
	tests := []struct {
		name    string
		args    args
		want    types.ServiceUpdateResponse
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UpdatePluginService(tt.args.serviceID, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdatePluginService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdatePluginService() = %v, want %v", got, tt.want)
			}
		})
	}
}
