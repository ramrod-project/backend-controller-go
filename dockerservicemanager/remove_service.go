package dockerservicemanager

import (
	"context"
	"log"
	"strconv"

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

	serv, _, err := dockerClient.ServiceInspectWithRaw(ctx, serviceID)
	if err != nil {
		return err
	}
	//update ports
	servIP, err := rethink.GetIPFromID(serviceID)
	if err != nil {
		log.Printf("%v", err)
	}
	log.Printf("serv id: \n%v", servIP)

	log.Printf("Removing service %v\n", serviceID)

	err = dockerClient.ServiceRemove(ctx, serviceID)
	if err != nil {
		return err
	}

	for _, port := range serv.Spec.EndpointSpec.Ports {
		err := rethink.RemovePort(servIP, strconv.FormatUint(uint64(port.PublishedPort), 10), port.Protocol)
		if err != nil {
			log.Printf("%v", err)
		}
	}

	return err
}
