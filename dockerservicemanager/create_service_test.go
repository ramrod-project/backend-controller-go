package dockerservicemanager

import (
	"context"
	"errors"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	types "github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
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
		err     error
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
					OS:      "posix",
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
				Warnings: nil,
			},
			wantErr: false,
			err:     nil,
		},
		{
			name: "Bad network",
			args: args{
				config: PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=5000",
						"PLUGIN=Harness",
					},
					Network: "blah",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    6000,
						PublishedPort: 6000,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "Harnessnet",
				},
			},
			want: types.ServiceCreateResponse{
				ID:       "",
				Warnings: nil,
			},
			wantErr: true,
			err:     errors.New("Error response from daemon: network blah not found"),
		},
		{
			name: "Duplicate service name",
			args: args{
				config: PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=5000",
						"PLUGIN=Harness",
					},
					Network: "test",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    5001,
						PublishedPort: 5001,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "Harness",
				},
			},
			want: types.ServiceCreateResponse{
				ID:       "",
				Warnings: nil,
			},
			wantErr: true,
			err:     errors.New("Error response from daemon: rpc error: code = Unknown desc = name conflicts with an existing object"),
		},
	}
	generatedIDs := make([]string, len(tests))
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreatePluginService(&tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePluginService() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				assert.Equal(t, err, tt.err)
			}
			assert.Equal(t, got.Warnings, tt.want.Warnings)
			log.Printf("Warnings: %v\n", got.Warnings)
			log.Printf("ID created: %v\n\n", got.ID)
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
		if v == "" {
			continue
		}
		log.Printf("Removing service %v\n", v)
		err := dockerClient.ServiceRemove(ctx, v)
		if err != nil {
			t.Errorf("%v", err)
		}
	}
}

func Test_generateServiceSpec(t *testing.T) {
	var (
		maxAttempts     = uint64(3)
		placementConfig = &swarm.Placement{}
		replicas        = uint64(1)
		second          = time.Second
	)

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = os.Getenv("TRAVIS_BRANCH")
	}
	if tag == "" {
		tag = "latest"
	}

	type args struct {
		config *PluginServiceConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *swarm.ServiceSpec
		wantErr bool
		err     error
	}{
		{
			name: "Good config",
			args: args{
				config: &PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=666",
						"PLUGIN=GoodPlugin",
					},
					Network: "goodnet",
					OS:      "all",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    666,
						PublishedPort: 666,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "GoodService",
				},
			},
			want: &swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "GoodService",
				},
				TaskTemplate: swarm.TaskSpec{
					ContainerSpec: swarm.ContainerSpec{
						DNSConfig: &swarm.DNSConfig{},
						Env: []string{
							"STAGE=DEV",
							"LOGLEVEL=DEBUG",
							"PORT=666",
							"PLUGIN=GoodPlugin",
						},
						Healthcheck: &container.HealthConfig{
							Interval: time.Second,
							Timeout:  time.Second * 3,
							Retries:  3,
						},
						Image:           "ramrodpcp/interpreter-plugin:" + tag,
						Labels:          make(map[string]string),
						Mounts:          nil,
						OpenStdin:       false,
						StopGracePeriod: &second,
						TTY:             false,
					},
					RestartPolicy: &swarm.RestartPolicy{
						Condition:   "on-failure",
						MaxAttempts: &maxAttempts,
					},
					Placement: placementConfig,
					Networks: []swarm.NetworkAttachmentConfig{
						swarm.NetworkAttachmentConfig{
							Target: "goodnet",
						},
					},
				},
				Mode: swarm.ServiceMode{
					Replicated: &swarm.ReplicatedService{
						Replicas: &replicas,
					},
				},
				UpdateConfig: &swarm.UpdateConfig{
					Parallelism: 0,
					Delay:       0,
				},
				EndpointSpec: &swarm.EndpointSpec{
					Mode: swarm.ResolutionModeVIP,
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    666,
						PublishedPort: 666,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
				},
			},
			wantErr: false,
			err:     nil,
		},
		{
			name: "Bad config OS",
			args: args{
				config: &PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=666",
						"PLUGIN=GoodPlugin",
					},
					Network: "goodnet",
					OS:      "dumb",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    666,
						PublishedPort: 666,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "BadService",
				},
			},
			want:    &swarm.ServiceSpec{},
			wantErr: true,
			err:     errors.New("invalid OS setting: dumb"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateServiceSpec(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateServiceSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				assert.Equal(t, err, tt.err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateServiceSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}
