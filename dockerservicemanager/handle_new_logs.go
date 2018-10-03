package dockerservicemanager

import (
	"bufio"
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/customtypes"
)

func newContainerLogger(ctx context.Context, dockerClient *client.Client, con types.ContainerJSON) (<-chan customtypes.ContainerLog, <-chan error) {
	logs := make(chan customtypes.ContainerLog)
	errs := make(chan error)

	go func() {
		defer close(logs)
		defer close(errs)

		logOut, err := dockerClient.ContainerAttach(ctx, con.ID, types.ContainerAttachOptions{
			Stream: true,
			Stderr: true,
			Stdout: true,
			Logs:   true,
		})
		defer logOut.Close()
		if err != nil {
			errs <- err
			return
		}

		// Get weird docker log header
		h := make([]byte, 8)
		n, err := logOut.Reader.Read(h)
		if err != nil {
			errs <- err
		} else if n == 0 {
			errs <- fmt.Errorf("nothing read")
		}

		scanner := bufio.NewScanner(logOut.Reader)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				break
			}

			new := scanner.Scan()
			if !new {
				time.Sleep(1000 * time.Millisecond)
			}

			con, err := dockerClient.ContainerInspect(ctx, con.ID)
			if err != nil {
				errs <- err
				return
			}

			logs <- customtypes.ContainerLog{
				ContainerID:   con.ID,
				ContainerName: con.Name,
				Log:           scanner.Text(),
				LogTimestamp:  int32(time.Now().Unix()),
			}
		}
	}()

	return logs, errs
}

// NewLogHandler takes the IDs from the log monitor
// and opens log readers for their corresponding containers
func NewLogHandler(ctx context.Context, newCons <-chan types.ContainerJSON) (<-chan (<-chan customtypes.ContainerLog), <-chan error) {
	ret := make(chan (<-chan customtypes.ContainerLog))
	errChans := make(chan (<-chan error))
	errs := make(chan error)

	// Log routine creator
	go func(in <-chan types.ContainerJSON) {
		defer close(ret)
		dockerClient, err := client.NewEnvClient()
		if err != nil {
			errs <- err
			close(errs)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case con, ok := <-in:
				if !ok {
					errs <- err
					close(errs)
					return
				}
				logger, errChan := newContainerLogger(ctx, dockerClient, con)
				errChans <- errChan
				ret <- logger
			}
		}
	}(newCons)

	// Error aggregator
	go func(in <-chan (<-chan error)) {
		defer close(errs)

		chans := []<-chan error{}

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
		}
	}(errChans)

	return ret, errs
}
