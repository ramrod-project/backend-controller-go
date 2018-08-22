package dockerservicemanager

import (
	"context"
	"log"

	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/rethink"
)

// RemovePluginService removes a service for a plugin
// given a service ID.
func RemovePluginService(serviceID string) error {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		return err
	}

	serv, _, _ := dockerClient.ServiceInspectWithRaw(ctx, serviceID)
	//update ports
	for _, port := range serv.Spec.EndpointSpec.Ports {
		rethink.RemovePort(string(port.PublishedPort), string(port.Protocol))
	}

	log.Printf("Removing service %v\n", serviceID)

	err = dockerClient.ServiceRemove(ctx, serviceID)

	return err
}
