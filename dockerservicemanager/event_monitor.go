package dockerservicemanager

import (
	"context"

	"github.com/docker/docker/api/types"
	events "github.com/docker/docker/api/types/events"
	client "github.com/docker/docker/client"
)

/*
	This function monitors events from the docker client
	and provides an event channel and an error channel
	for other consumers. Right now, all events are passed
	and must be parsed/handled by the rethink EventUpdate
	routine.
*/
func EventMonitor() (<-chan events.Message, <-chan error) {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		panic(err)
	}

	eventChan, errChan := dockerClient.Events(ctx, types.EventsOptions{})

	return eventChan, errChan
}
