package dockerservicemanager

import (
	"context"
	"regexp"

	"github.com/docker/docker/api/types"
	events "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
)

var imageRegex = regexp.MustCompile(`^ramrodpcp.*?`)
var stoppedRegex = regexp.MustCompile(`(stopped|dead)`)

func newLogFilter() filters.Args {
	// Filter plugin containers (start events)
	logFilter := filters.NewArgs()
	logFilter.Add("type", "service")
	logFilter.Add("event", "create")

	return logFilter
}

func stackServices(ctx context.Context, dockerClient *client.Client) ([]swarm.Service, error) {

	services, err := dockerClient.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return []swarm.Service{}, err
	}

	ret := make([]swarm.Service, len(services))

	for i, svc := range services {
		insp, _, err := dockerClient.ServiceInspectWithRaw(ctx, svc.ID)
		if err != nil {
			return []swarm.Service{}, err
		}
		ret[i] = insp
	}

	return ret, nil
}

// NewLogMonitor returns a channel of container objects
// for new containers that start.
func NewLogMonitor(ctx context.Context) (<-chan swarm.Service, <-chan error) {
	ret := make(chan swarm.Service)
	errs := make(chan error)

	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	// Filter plugin containers (start events)
	logFilter := newLogFilter()

	svcStart, errSvcStart := dockerClient.Events(ctx, types.EventsOptions{
		Filters: logFilter,
	})

	go func(in <-chan events.Message) {
		defer close(ret)
		defer close(errs)
		// Get initial containers in stack
		stackSvcs, err := stackServices(ctx, dockerClient)
		if err != nil {
			panic(err)
		}

		for _, svc := range stackSvcs {
			ret <- svc
		}

		for {
			select {
			case <-ctx.Done():
				return
			case e := <-errSvcStart:
				errs <- e
			case n := <-in:
				svc, _, err := dockerClient.ServiceInspectWithRaw(ctx, n.Actor.ID)
				if err != nil {
					errs <- err
					break
				}
				ret <- svc
			}
		}
	}(svcStart)

	return ret, errSvcStart
}
