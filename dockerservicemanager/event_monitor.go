package dockerservicemanager

import (
	"context"

	"github.com/docker/docker/api/types"
	events "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	client "github.com/docker/docker/client"
)

func eventFanIn(eventChans []<-chan events.Message, errChans []<-chan error) (<-chan events.Message, <-chan error) {
	evts := make(chan events.Message)
	errs := make(chan error)

	// Collect errors
	for _, errChan := range errChans {
		go func(c <-chan error) {
			for err := range c {
				errs <- err
			}
		}(errChan)
	}

	// Collect messages
	for _, eventChan := range eventChans {
		go func(c <-chan events.Message) {
			for evt := range c {
				evts <- evt
			}
		}(eventChan)
	}

	return evts, errs
}

// EventMonitor monitors events from the docker client
// and provides an event channel and an error channel
// for other consumers. Right now, all events are passed
// and must be parsed/handled by the rethink EventUpdate
// routine.
func EventMonitor() (<-chan events.Message, <-chan error) {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	// Filter plugin containers (start and die events)
	containerFilter := filters.NewArgs()
	containerFilter.Add("type", "container")
	containerFilter.Add("image", "ramrodpcp/interpreter-plugin")
	containerFilter.Add("image", "ramrodpcp/interpreter-plugin-windows")
	containerFilter.Add("image", "ramrodpcp/auxiliary-services")
	containerFilter.Add("event", "die")
	containerFilter.Add("event", "health_status")

	// Filter service update events
	serviceFilter := filters.NewArgs()
	serviceFilter.Add("type", "service")
	containerFilter.Add("event", "update")
	containerFilter.Add("event", "create")
	containerFilter.Add("event", "remove")

	containerChan, errContainerChan := dockerClient.Events(ctx, types.EventsOptions{
		Filters: containerFilter,
	})

	serviceChan, errServiceChan := dockerClient.Events(ctx, types.EventsOptions{
		Filters: serviceFilter,
	})

	eventChan, errChan := eventFanIn(
		[]<-chan events.Message{containerChan, serviceChan},
		[]<-chan error{errContainerChan, errServiceChan},
	)

	return eventChan, errChan
}
