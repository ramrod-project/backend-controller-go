package dockerservicemanager

import (
	"context"

	client "github.com/docker/docker/client"
)

// RemovePluginService removes a service for a plugin
// given a service ID.
func RemovePluginService(serviceID string) error {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		return err
	}

	// log.Printf("Removing service %v\n", serviceID)

	err = dockerClient.ServiceRemove(ctx, serviceID)

	return err
}
