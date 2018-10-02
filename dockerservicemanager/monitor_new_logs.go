package dockerservicemanager

import (
	"context"
	"regexp"

	"github.com/docker/docker/api/types"
	events "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	client "github.com/docker/docker/client"
)

/*
Pseudocode:

NewLogFilter
filterArgs for: container start event of stack/plugin/aux images

stackContainerIDs
takes: nil
returns: []string (list of stack container IDs)

	query docker for list of services
	filter by stack members (com.whatever.something.stackid)
	return list of container IDs

NewLogMonitor
takes: context
returns: <-chan string (container IDs), <-chan error

	create channel for container IDs

	(goroutine)
		query docker for stack member logs
		push out stack member container IDs over channel
		get docker events channel (using NewLogFilter)
		while forever
			receive event
			parse contianer ID
			chan <- ID

	return channel

*/

var imageRegex = regexp.MustCompile(`^ramrodpcp.*?`)
var stoppedRegex = regexp.MustCompile(`(stopped|dead)`)

func newLogFilter() filters.Args {
	// Filter plugin containers (start events)
	logFilter := filters.NewArgs()
	logFilter.Add("type", "container")
	logFilter.Add("image", "ramrodpcp/interpreter-plugin")
	logFilter.Add("image", "ramrodpcp/interpreter-plugin-extra")
	logFilter.Add("image", "ramrodpcp/auxiliary-services")
	logFilter.Add("image", "ramrodpcp/auxiliary-wrapper")
	logFilter.Add("image", "ramrodpcp/database-brain")
	logFilter.Add("image", "ramrodpcp/backend-controller")
	logFilter.Add("image", "ramrodpcp/frontend-ui")
	logFilter.Add("event", "start")

	return logFilter
}

func stackContainerIDs(ctx context.Context, dockerClient *client.Client) ([]string, error) {

	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return []string{}, err
	}

	containerIDs := make([]string, 0, len(containers))

	for _, container := range containers {
		if imageRegex.Match([]byte(container.Image)) && !stoppedRegex.Match([]byte(container.State)) {
			containerIDs = append(containerIDs, container.Names[0])
		}
	}
	return containerIDs, nil
}

// NewLogMonitor returns a channel of strings that
// will container new container IDs of containers
// that started.
func NewLogMonitor(ctx context.Context) (<-chan string, <-chan error) {
	ret := make(chan string)

	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	// Filter plugin containers (start events)
	logFilter := newLogFilter()

	containerStart, errContainerStart := dockerClient.Events(ctx, types.EventsOptions{
		Filters: logFilter,
	})

	go func(in <-chan events.Message) {
		defer close(ret)
		// Get initial containers in stack
		stackContainers, err := stackContainerIDs(ctx, dockerClient)
		if err != nil {
			panic(err)
		}

		for _, id := range stackContainers {
			ret <- id
		}

		for {
			select {
			case <-ctx.Done():
				return
			case n := <-in:
				if v, ok := n.Actor.Attributes["name"]; ok {
					ret <- v
				}
			}
		}
	}(containerStart)

	return ret, errContainerStart
}
