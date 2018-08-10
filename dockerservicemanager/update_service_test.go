package dockerservicemanager

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

func SpecToString(s *swarm.ServiceSpec) string {
	str := fmt.Sprintf(`
			Name: %v,
			TaskTemplate:
				ContainerSpec:
					DNSConfig: %v
					Env: %v
					Image: %v
					Labels: %v
					Mounts: %v
				RestartPolicy:
					Condition: %v
					MaxAttempts: %v
				Placement:
					Constraints: %v
				Networks:
					Target: %v
				Mode:
					Replicas: %v
				EndpointSpec:
					Mode: %v
					Ports: %v
		`,
		s.Annotations.Name,
		s.TaskTemplate.ContainerSpec.DNSConfig,
		s.TaskTemplate.ContainerSpec.Env,
		s.TaskTemplate.ContainerSpec.Image,
		s.TaskTemplate.ContainerSpec.Labels,
		s.TaskTemplate.ContainerSpec.Mounts,
		s.TaskTemplate.RestartPolicy.Condition,
		s.TaskTemplate.RestartPolicy.MaxAttempts,
		s.TaskTemplate.Placement.Constraints,
		s.TaskTemplate.Networks,
		*s.Mode.Replicated.Replicas,
		s.EndpointSpec.Mode,
		s.EndpointSpec.Ports,
	)
	return str
}

func TestUpdatePluginService(t *testing.T) {
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

	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	netRes, err := dockerClient.NetworkCreate(ctx, "test_update", types.NetworkCreate{
		Driver:     "overlay",
		Attachable: true,
	})
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	serviceSpec := &swarm.ServiceSpec{
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
					"PLUGIN=Harness",
				},
				Healthcheck: &container.HealthConfig{
					Interval: time.Second,
					Timeout:  time.Second * 3,
					Retries:  3,
				},
				Image:           "ramrodpcp/interpreter-plugin:" + tag,
				StopGracePeriod: &second,
			},
			RestartPolicy: &swarm.RestartPolicy{
				Condition:   "on-failure",
				MaxAttempts: &maxAttempts,
			},
			Placement: placementConfig,
			Networks: []swarm.NetworkAttachmentConfig{
				swarm.NetworkAttachmentConfig{
					Target: "test_update",
				},
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicas,
			},
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
	}

	resp, err := dockerClient.ServiceCreate(ctx, *serviceSpec, types.ServiceCreateOptions{})
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	id := resp.ID

	type args struct {
		config *PluginServiceConfig
		id     string
	}
	tests := []struct {
		name       string
		args       args
		want       types.ServiceUpdateResponse
		wantErr    bool
		inspectRes swarm.Service
	}{
		{
			name: "Valid service update",
			args: args{
				config: &PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=5000",
						"PLUGIN=Harness",
						"TEST=TEST",
					},
					Network: "test_update",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    666,
						PublishedPort: 666,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "GoodService",
				},
				id: id,
			},
			want: types.ServiceUpdateResponse{
				Warnings: nil,
			},
			wantErr: false,
		},
		{
			name: "Bad ID",
			args: args{
				config: &PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=5000",
						"PLUGIN=Harness",
						"TEST=TEST",
					},
					Network: "test_update",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    666,
						PublishedPort: 666,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "GoodService",
				},
				id: "",
			},
			want: types.ServiceUpdateResponse{
				Warnings: nil,
			},
			wantErr: true,
		},
		{
			name: "Invalid service name change",
			args: args{
				config: &PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=5000",
						"PLUGIN=Harness",
						"TEST=TEST",
					},
					Network: "test_update",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    0,
						PublishedPort: 0,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "BadServiceUpdate",
				},
				id: id,
			},
			want: types.ServiceUpdateResponse{
				Warnings: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			time.Sleep(time.Second)
			got, err := UpdatePluginService(tt.args.id, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdatePluginService() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				return
			}
			_, _, err = dockerClient.ServiceInspectWithRaw(ctx, tt.args.id)
			if err != nil {
				t.Errorf("%v", err)
				return
			} else {
				time.Sleep(time.Second)
			}
			assert.Equal(t, tt.want, got)
		})
	}

	// Docker cleanup
	err = dockerClient.ServiceRemove(ctx, id)
	if err != nil {
		t.Errorf("%v", err)
	}
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
	for _, c := range containers {
		if c.ID == "" {
			continue
		}
		err := dockerClient.ContainerKill(ctx, c.ID, "SIGKILL")
		if err != nil {
			t.Errorf("%v", err)
		}
		err = dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			t.Errorf("%v", err)
		}
	}
	start := time.Now()
	for time.Since(start) < 10*time.Second {
		dockerClient.NetworkRemove(ctx, netRes.ID)
		time.Sleep(time.Second)
		_, err := dockerClient.NetworkInspect(ctx, netRes.ID)
		if err != nil {
			_, err := dockerClient.NetworksPrune(ctx, filters.Args{})
			if err != nil {
				break
			}
		}
	}
}

func Test_checkReady(t *testing.T) {
	type args struct {
		ctx          context.Context
		dockerClient *client.Client
		serviceID    string
	}
	tests := []struct {
		name    string
		args    args
		want    uint64
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkReady(tt.args.ctx, tt.args.dockerClient, tt.args.serviceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkReady() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("checkReady() = %v, want %v", got, tt.want)
			}
		})
	}
}
