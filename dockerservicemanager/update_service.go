package dockerservicemanager

import (
	"context"
	"fmt"
	"time"

	"github.com/ramrod-project/backend-controller-go/rethink"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
)

/*func generateUpdateSpec(ctx context.Context, dockerClient *client.Client, id string, config *PluginServiceConfig) (*swarm.ServiceSpec, uint64, error) {

	log.Printf("Updating service %v with config %v\n", id, config)

	log.Printf("Inspecting service %v\n", id)
	inspectResults, _, err := dockerClient.ServiceInspectWithRaw(ctx, id)
	if err != nil {
		return &swarm.ServiceSpec{}, 0, err
	} else if inspectResults.UpdateStatus.State == swarm.UpdateStateUpdating {
		return &swarm.ServiceSpec{}, 0, fmt.Errorf("service %v already updating; cannot update", id)
	}
	log.Printf("Got service: %v version: %v", inspectResults.Spec, inspectResults.Meta.Version)

	log.Printf("Creating updated service spec\n")
	serviceSpec := inspectResults.Spec
	version := inspectResults.Meta.Version.Index

	if config.Environment != nil && !reflect.DeepEqual(serviceSpec.TaskTemplate.ContainerSpec.Env, config.Environment) {
		serviceSpec.TaskTemplate.ContainerSpec.Env = config.Environment
		log.Printf("Assigned environment %v", config.Environment)
	}

	if config.Volumes != nil && !reflect.DeepEqual(serviceSpec.TaskTemplate.ContainerSpec.Mounts, config.Volumes) {
		serviceSpec.TaskTemplate.ContainerSpec.Mounts = config.Volumes
		log.Printf("Assigning Mounts %v", config.Volumes)
	}

	osString := "node.labels.os==" + string(config.OS)
	if len(serviceSpec.TaskTemplate.Placement.Constraints) == 0 {
		log.Printf("No constraints")
	} else if serviceSpec.TaskTemplate.Placement.Constraints[0] != osString && config.OS != rethink.PluginOSAll {
		serviceSpec.TaskTemplate.Placement.Constraints[0] = osString
		log.Printf("Assigned OS %v", osString)
	}

	if !reflect.DeepEqual(serviceSpec.EndpointSpec.Ports, config.Ports) {
		serviceSpec.EndpointSpec.Ports = config.Ports
		log.Printf("Assigned Ports %v", config.Ports)
	}

	return &serviceSpec, version, nil
}*/

func checkReady(ctx context.Context, dockerClient *client.Client, serviceID string) (uint64, error) {

	start := time.Now()
	for {
		if time.Since(start) > 10*time.Second {
			break
		}
		inspectResults, _, err := dockerClient.ServiceInspectWithRaw(ctx, serviceID)
		if err != nil {
			return 0, err
		}
		if inspectResults.UpdateStatus.State != swarm.UpdateStateUpdating {
			return inspectResults.Version.Index, nil
		}
		time.Sleep(time.Second)
	}
	return 0, fmt.Errorf("timeout: service %v still updating", serviceID)
}

// UpdatePluginService updates a given service by ID string
// and given a valid PluginServiceConfig. It will attempt
// to update the service and relaunch it.
func UpdatePluginService(serviceID string, config *PluginServiceConfig) (types.ServiceUpdateResponse, error) {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		return types.ServiceUpdateResponse{}, err
	}

	serviceSpec, err := generateServiceSpec(config)
	if err != nil {
		return types.ServiceUpdateResponse{}, err
	}

	version, err := checkReady(ctx, dockerClient, serviceID)
	if err != nil {
		return types.ServiceUpdateResponse{}, err
	}

	resp, err := dockerClient.ServiceUpdate(ctx, serviceID, swarm.Version{Index: version}, *serviceSpec, types.ServiceUpdateOptions{})
	for _, port := range config.Ports {
		rethink.RemovePort(config.Address, string(port.PublishedPort), port.Protocol)
		rethink.AddPort(config.Address, string(port.PublishedPort), port.Protocol)
	}

	return resp, err
}
