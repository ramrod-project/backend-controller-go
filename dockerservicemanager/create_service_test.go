package dockerservicemanager

import (
	"context"
	"log"
	"reflect"
	"testing"

	types "github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
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
		{
			name: "Test creating a plugin service",
			args: args{
				config: PluginServiceConfig{
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
				},
			},
			want: types.ServiceCreateResponse{
				ID:       "",
				Warnings: []string{},
			},
			wantErr: false,
		},
	}
	generatedIDs := make([]string, len(tests))
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreatePluginService(&tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePluginService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.Warnings, tt.want.Warnings) {
				t.Errorf("CreatePluginService() = %v, want %v", got.Warnings, tt.want.Warnings)
			}
			log.Printf("ID created: %v\n", got.ID)
			generatedIDs[i] = got.ID
		})
	}
	// Docker cleanup
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
	}
	for _, v := range generatedIDs {
		log.Printf("Removing service %v\n", v)
		err := dockerClient.ServiceRemove(ctx, v)
		if err != nil {
			t.Errorf("%v", err)
		}
	}
}

func Test_generateServiceSpec(t *testing.T) {
	type args struct {
		config *PluginServiceConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *swarm.ServiceSpec
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateServiceSpec(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateServiceSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateServiceSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}
