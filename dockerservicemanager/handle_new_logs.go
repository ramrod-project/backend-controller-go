package dockerservicemanager

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/customtypes"
)

func newLogger(ctx context.Context, dockerClient *client.Client, svc swarm.Service) (<-chan customtypes.Log, <-chan error) {
	logs := make(chan customtypes.Log)
	errs := make(chan error)

	go func() {
		defer close(logs)
		defer close(errs)

		// Set vars because this container's info
		// won't change throughout its lifespan
		svcID := svc.ID
		svcName := svc.Spec.Annotations.Name

		logOut, err := dockerClient.ServiceLogs(ctx, svcID, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
		})

		if err != nil {
			errs <- err
			return
		}
		defer logOut.Close()
		// Get weird docker log header
		h := make([]byte, 8)
		n, err := logOut.Read(h)
		if err != nil {
			errs <- err
		} else if n == 0 {
			errs <- fmt.Errorf("nothing read")
		}

		scanner := bufio.NewScanner(logOut)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				break
			}

			new := scanner.Scan()
			if new {
				logs <- customtypes.Log{
					Log:          scanner.Text(),
					//LogTimestamp: uint64(time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))),
					LogTimestamp: int64(time.Now().UnixNano()) / 1000000,
					ServiceName:  svcName,
				}
				continue
			}

			time.Sleep(1000 * time.Millisecond)
		}
	}()

	return logs, errs
}

// NewLogHandler takes the IDs from the log monitor
// and opens log readers for their corresponding containers
func NewLogHandler(ctx context.Context, newSvcs <-chan swarm.Service) (<-chan (<-chan customtypes.Log), <-chan error) {
	ret := make(chan (<-chan customtypes.Log))
	errChans := make(chan (<-chan error))
	errs := make(chan error)
	logErrs := make(chan error)

	// Log routine creator
	go func(in <-chan swarm.Service) {
		defer close(ret)
		defer close(logErrs)
		dockerClient, err := client.NewEnvClient()
		if err != nil {
			logErrs <- err
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case con, ok := <-in:
				if !ok {
					logErrs <- err
					return
				}
				logger, errChan := newLogger(ctx, dockerClient, con)
				errChans <- errChan
				ret <- logger
			}
		}
	}(newSvcs)

	// Error aggregator
	go func(in <-chan (<-chan error)) {
		defer close(errs)

		chans := []<-chan error{logErrs}

		for {

			select {
			case <-ctx.Done():
				return
			case c, ok := <-in:
				if !ok {
					return
				}
				chans = append(chans, c)
			default:
				break
			}

			for i, c := range chans {
				select {
				case e, ok := <-c:
					if !ok {
						chans = append(chans[:i], chans[i+1:]...)
						i--
					} else if e != nil {
						errs <- e
					}
				default:
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}(errChans)

	return ret, errs
}
