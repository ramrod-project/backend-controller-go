package dockerservicemanager

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

/*
Pseudocode:

NewLogHandler
takes: <-chan string (from log monitor)
returns: <-chan chan string (for the aggregator), <-chan errors
	create channels
	(goroutine)
		receive new ID
		create chan string
		spawn goroutine
		(goroutine)
			create log reader for the container
			read from log forever
			send log strings back over channel
		chan chan string <- chan string
	return channels
*/

func newContainerLogger(ctx context.Context, dockerClient *client.Client, id string) (<-chan string, <-chan error) {
	logs := make(chan string)
	errs := make(chan error)

	go func() {
		defer close(logs)
		defer close(errs)

		logOut, err := dockerClient.ContainerLogs(ctx, id, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Timestamps: true,
			Follow:     true,
		})
		if err != nil {
			errs <- err
			return
		}

		buf := new(bytes.Buffer)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				break
			}

			i, err := buf.ReadFrom(logOut)
			if err != nil {
				if err == io.EOF {
					return
				}
				errs <- err
				continue
			} else if i == 0 {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			logs <- buf.String()
			buf.Reset()
		}

	}()

	return logs, errs
}

// NewLogHandler takes the IDs from the log monitor
// and opens log readers for their corresponding containers
func NewLogHandler(ctx context.Context, newIDs <-chan string) (<-chan (<-chan string), <-chan error) {
	ret := make(chan (<-chan string))
	errChans := make(chan (<-chan error))
	errs := make(chan error)

	// Log routine creator
	go func(in <-chan string) {
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
			case id := <-in:
				logger, errChan := newContainerLogger(ctx, dockerClient, id)
				errChans <- errChan
				ret <- logger
			}
		}
	}(newIDs)

	// Error aggregator
	go func(in <-chan (<-chan error)) {
		defer close(errs)

		chans := []<-chan error{}

		for {

			select {
			case <-ctx.Done():
				return
			case c := <-in:
				chans = append(chans, c)
			default:
				break
			}

			for _, c := range chans {
				select {
				case e := <-c:
					errs <- e
				default:
					break
				}
			}
		}
	}(errChans)

	return ret, errs
}
