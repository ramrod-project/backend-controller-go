package dockerservicemanager

import (
	"context"

	"github.com/docker/docker/api/types"
	events "github.com/docker/docker/api/types/events"
	client "github.com/docker/docker/client"
)

func EventMonitor() (<-chan events.Message, <-chan error) {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		panic(err)
	}

	eventChan, errChan := dockerClient.Events(ctx, types.EventsOptions{})

	return eventChan, errChan
}
