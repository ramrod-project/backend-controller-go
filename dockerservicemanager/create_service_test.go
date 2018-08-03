package dockerservicemanager

import (
	"reflect"
	"testing"

	types "github.com/docker/docker/api/types"
)

func TestCreatePluginService(t *testing.T) {
	type args struct {
		config PluginServiceConfig
	}
	tests := []struct {
		name    string
		args    args
		want    types.ServiceCreateResponse
		wantErr bool
	}{
		// TODO: Add test cases.
		/*
			config := &dockerservicemanager.PluginServiceConfig{
				Environment: []string{
					"STAGE=DEV",
					"LOGLEVEL=DEBUG",
					"PORT=5000",
					"PLUGIN=Harness",
				},
				Network: "test",
				OS:      "linux",
				Ports: []swarm.PortConfig{swarm.PortConfig{
					Protocol:      swarm.PortConfigProtocolTCP,
					TargetPort:    5000,
					PublishedPort: 5000,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				}},
				ServiceName: "Harness",
			}

			res, err := dockerservicemanager.CreatePluginService(config)
			if err != nil {
				panic(err)
			}
			log.Printf("Response: %v\n", res)
		*/
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreatePluginService(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePluginService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreatePluginService() = %v, want %v", got, tt.want)
			}
		})
	}
}
