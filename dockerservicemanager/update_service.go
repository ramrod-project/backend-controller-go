package dockerservicemanager

import (
	"context"
	"log"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
)

// UpdatePluginService updates a given service by ID string
// and given a valid PluginServiceConfig. It will attempt
// to update the service and relaunch it.
func UpdatePluginService(serviceID string, config *PluginServiceConfig) (types.ServiceUpdateResponse, error) {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		return types.ServiceUpdateResponse{}, err
	}

	log.Printf("Updating service %v with new config %v\n", serviceID, config)

	serviceSpec, err := generateServiceSpec(config)

	if err != nil {
		return types.ServiceUpdateResponse{}, err
	}

	swarmVersion := swarm.Version{
		Index: uint64(1),
	}

	resp, err := dockerClient.ServiceUpdate(ctx, serviceID, swarmVersion, *serviceSpec, types.ServiceUpdateOptions{})

	return resp, err
}
