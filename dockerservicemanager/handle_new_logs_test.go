package dockerservicemanager

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/customtypes"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
)

func startTestContainers(ctx context.Context, number int) error {
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	testServiceSpec.TaskTemplate.ContainerSpec.Command = []string{"/bin/sh", "-c", "while true; do echo 'test'; sleep 1; done"}

	for i := 0; i < number; i++ {
		testServiceSpec.Name = "TestService" + strconv.Itoa(i)
		_, err = dockerClient.ServiceCreate(ctx, testServiceSpec, types.ServiceCreateOptions{})
		if err != nil {
			log.Printf("%v", err)
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

func checkContainerIDs(ctx context.Context, number int) (<-chan []string, <-chan error) {
	ret := make(chan []string)
	errs := make(chan error)
	containerNames := make([]string, number+1)

	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	go func() {
		defer close(ret)
		defer close(errs)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				break
			}
			cons, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
			if err != nil {
				errs <- err
				return
			}
			if len(cons) == number+1 {
				for i := range containerNames {
					containerNames[i] = cons[i].ID
				}
				ret <- containerNames
				return
			}
			time.Sleep(1000 * time.Millisecond)
		}
	}()
	return ret, errs
}

func Test_newContainerLogger(t *testing.T) {

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = "latest"
	}

	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// Set up clean environment
	if err := test.DockerCleanUp(ctx, dockerClient, ""); err != nil {
		t.Errorf("setup error: %v", err)
		return
	}

	tests := []struct {
		name    string
		run     func(context.Context) ([]string, error)
		timeout time.Duration
	}{
		{
			name: "test 1",
			run: func(ctx context.Context) ([]string, error) {
				err := startTestContainers(ctx, 1)
				if err != nil {
					return []string{}, err
				}

				res, errs := checkContainerIDs(ctx, 0)

				select {
				case <-ctx.Done():
					return []string{}, fmt.Errorf("timeout context exceeded")
				case err = <-errs:
					return []string{}, fmt.Errorf("%v", err)
				case r := <-res:
					return r, nil
				}
			},
			timeout: 20 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			timeoutCtx, cancel := context.WithTimeout(ctx, tt.timeout)
			defer cancel()
			defer test.DockerCleanUp(ctx, dockerClient, "")

			cons, err := tt.run(timeoutCtx)
			if err != nil {
				t.Errorf("%v", err)
				return
			}

			out, errs := newContainerLogger(timeoutCtx, dockerClient, cons[0])

			for {
				select {
				case <-timeoutCtx.Done():
					t.Errorf("timeout context exceeded")
					return
				case e := <-errs:
					t.Errorf("%v", e)
					return
				case o := <-out:
					log.Printf("%+v", o)
					assert.True(t, len(o.Log) > 0)
					return
				}
			}

		})
	}
}

func TestNewLogHandler(t *testing.T) {
	type args struct {
		ctx      context.Context
		newNames <-chan string
	}
	tests := []struct {
		name  string
		args  args
		want  <-chan (<-chan customtypes.ContainerLog)
		want1 <-chan error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

		})
	}
}
