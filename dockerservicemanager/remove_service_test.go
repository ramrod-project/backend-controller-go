package dockerservicemanager

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

func TestRemovePluginService(t *testing.T) {
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

	netRes, err := dockerClient.NetworkCreate(ctx, "test", types.NetworkCreate{
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
					Target: "test",
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

	type args struct {
		serviceID string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		err     error
	}{
		{
			name: "Shutdown existing service",
			args: args{
				serviceID: resp.ID,
			},
			wantErr: false,
		},
		{
			name: "Non existing service",
			args: args{
				serviceID: "whatisthis",
			},
			wantErr: true,
			err:     errors.New("Error response from daemon: service whatisthis not found"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RemovePluginService(tt.args.serviceID); (err != nil) != tt.wantErr {
				t.Errorf("RemovePluginService() error = %v, wantErr %v", err, tt.wantErr)
			} else if tt.wantErr {
				assert.Equal(t, tt.err, err)
			}
		})
	}
	dockerClient.NetworkRemove(ctx, netRes.ID)
}
