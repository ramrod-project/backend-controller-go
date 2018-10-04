package dockerservicemanager

import (
	"context"
	"regexp"

	"github.com/docker/docker/api/types"
	events "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	client "github.com/docker/docker/client"
)

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

func stackContainerIDs(ctx context.Context, dockerClient *client.Client) ([]types.ContainerJSON, error) {

	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return []types.ContainerJSON{}, err
	}

	ret := make([]types.ContainerJSON, len(containers))

	for i, con := range containers {
		insp, err := dockerClient.ContainerInspect(ctx, con.ID)
		if err != nil {
			return []types.ContainerJSON{}, err
		}
		ret[i] = insp
	}

	return ret, nil
}

// NewLogMonitor returns a channel of container objects
// for new containers that start.
func NewLogMonitor(ctx context.Context) (<-chan types.ContainerJSON, <-chan error) {
	ret := make(chan types.ContainerJSON)
	errs := make(chan error)

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
		defer close(errs)
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
			case e := <-errContainerStart:
				errs <- e
			case n := <-in:
				con, err := dockerClient.ContainerInspect(ctx, n.ID)
				if err != nil {
					errs <- err
					break
				}
				ret <- con
			}
		}
	}(containerStart)

	return ret, errContainerStart
}
