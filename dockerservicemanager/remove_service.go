package dockerservicemanager

import (
	"context"
	"log"

	client "github.com/docker/docker/client"
)

// RemovePluginService removes a service for a plugin
// given a service ID.
func RemovePluginService(serviceID string) error {
	log.Printf("removing: %v", serviceID)
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		return err
	}

	log.Printf("Removing service %v\n", serviceID)

	err = dockerClient.ServiceRemove(ctx, serviceID)

	return err
}
