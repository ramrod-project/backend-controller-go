package dockerservicemanager

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
)

func TestUpdatePluginService(t *testing.T) {
	env := os.Getenv("STAGE")
	os.Setenv("STAGE", "TESTING")
	var (
		maxAttempts     = uint64(3)
		placementConfig = &swarm.Placement{}
		replicas        = uint64(1)
		second          = time.Second
	)

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = "latest"
	}

	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// Set up clean environment
	if err := test.DockerCleanUp(ctx, dockerClient, ""); err != nil {
		t.Errorf("setup error: %v", err)
	}

	session, brainID, err := test.StartBrain(ctx, t, dockerClient, test.BrainSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	testPort := map[string]interface{}{
		"Interface":    GetManagerIP(),
		"TCPPorts":     []string{},
		"UDPPorts":     []string{},
		"NodeHostName": "ubuntu",
		"OS":           "posix",
	}
	_, err = r.DB("Controller").Table("Ports").Insert(testPort).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	netID, err := test.CheckCreateNet("test_update")
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
					Address:     GetManagerIP(),
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
					Address:     GetManagerIP(),
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
					Address:     GetManagerIP(),
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

	test.KillService(ctx, dockerClient, brainID)

	//Docker cleanup
	if err := test.DockerCleanUp(ctx, dockerClient, netID); err != nil {
		t.Errorf("cleanup error: %v", err)
	}

	os.Setenv("STAGE", env)
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
