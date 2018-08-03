package dockerservicemanager

import (
	"context"
	"log"
	"time"

	types "github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	mount "github.com/docker/docker/api/types/mount"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
)

// PluginServiceConfig contains configuration parameters
// for a plugin service.
type PluginServiceConfig struct {
	Environment []string
	Network     string
	OS          string
	Ports       []swarm.PortConfig `json:",omitempty"`
	ServiceName string
	Volumes     []mount.Mount `json:",omitempty"`
}

func generateServiceSpec(config *PluginServiceConfig) (*swarm.ServiceSpec, error) {
	var (
		labels          = make(map[string]string)
		imageName       string
		placementConfig = &swarm.Placement{}
		maxAttempts     uint64
		stopGrace       time.Duration
		replicas        uint64
	)

	log.Printf("Creating service %v with config %v\n", config.ServiceName, config)

	maxAttempts = uint64(3)
	replicas = uint64(1)
	stopGrace = time.Second

	// Determine container image
	if config.OS == "linux" || config.OS == "all" {
		imageName = "ramrodpcp/interpreter-plugin:dev"
	} else {
		imageName = "ramrodpcp/interpreter-plugin-windows:dev"
		constraints := []string{"node.labels.os==nt"}
		placementConfig.Constraints = constraints
	}

	log.Printf("Creating service spec\n")

	serviceSpec := &swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: config.ServiceName,
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				DNSConfig: &swarm.DNSConfig{},
				Env:       config.Environment,
				Healthcheck: &container.HealthConfig{
					Interval: time.Second,
					Timeout:  time.Second * 3,
					Retries:  3,
				},
				Image:           imageName,
				Labels:          labels,
				Mounts:          config.Volumes,
				OpenStdin:       false,
				StopGracePeriod: &stopGrace,
				TTY:             false,
			},
			RestartPolicy: &swarm.RestartPolicy{
				Condition:   "on-failure",
				MaxAttempts: &maxAttempts,
			},
			Placement: placementConfig,
			Networks: []swarm.NetworkAttachmentConfig{
				swarm.NetworkAttachmentConfig{
					Target: config.Network,
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
			Mode:  swarm.ResolutionModeVIP,
			Ports: config.Ports,
		},
	}

	return serviceSpec, nil
}

// CreatePluginService creates a service for a plugin
// given a PluginServiceConfig.
func CreatePluginService(config *PluginServiceConfig) (types.ServiceCreateResponse, error) {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		return types.ServiceCreateResponse{}, err
	}

	log.Printf("Generating service spec\n")

	serviceSpec, err := generateServiceSpec(config)

	if err != nil {
		return types.ServiceCreateResponse{}, err
	}

	log.Printf("Service spec created: %v", serviceSpec)

	resp, err2 := dockerClient.ServiceCreate(ctx, *serviceSpec, types.ServiceCreateOptions{})
	return resp, err2

}
