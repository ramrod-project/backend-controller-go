package dockerservicemanager

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
)

var rethinkRegex = regexp.MustCompile(`rethinkdb`)

func startContainers(ctx context.Context, number int) error {
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	newPluginSpec := &swarm.ServiceSpec{}
	*newPluginSpec = test.GenericPluginConfig

	for i := 0; i < number; i++ {
		newPluginSpec.Name = "testservice" + strconv.Itoa(i)
		newPluginSpec.EndpointSpec.Ports[0].TargetPort = newPluginSpec.EndpointSpec.Ports[0].TargetPort + 1
		newPluginSpec.EndpointSpec.Ports[0].PublishedPort = newPluginSpec.EndpointSpec.Ports[0].PublishedPort + 1
		_, err = dockerClient.ServiceCreate(ctx, *newPluginSpec, types.ServiceCreateOptions{})
		if err != nil {
			log.Printf("%v", err)
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

func checkContainers(ctx context.Context, number int) (<-chan []types.ContainerJSON, <-chan error) {
	ret := make(chan []types.ContainerJSON)
	errs := make(chan error)
	containerNames := make([]types.ContainerJSON, number+1)

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
					con, err := dockerClient.ContainerInspect(ctx, cons[i].ID)
					if err != nil {
						errs <- err
						return
					}
					containerNames[i] = con
				}
				ret <- containerNames
				return
			}
			time.Sleep(1000 * time.Millisecond)
		}
	}()
	return ret, errs
}

func Test_stackContainerIDs(t *testing.T) {

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

	netID, err := test.CheckCreateNet("testnet")
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	newSpec := &swarm.ServiceSpec{}
	*newSpec = test.BrainSpec

	newSpec.Networks = []swarm.NetworkAttachmentConfig{
		swarm.NetworkAttachmentConfig{
			Target:  netID,
			Aliases: []string{"rethinkdb"},
		},
	}

	test.GenericPluginConfig.TaskTemplate.Networks = []swarm.NetworkAttachmentConfig{
		swarm.NetworkAttachmentConfig{
			Target: netID,
		},
	}

	tests := []struct {
		name    string
		run     func(context.Context) ([]types.ContainerJSON, error)
		timeout time.Duration
		wantErr bool
	}{
		{
			name: "get nothing",
			run: func(ctx context.Context) ([]types.ContainerJSON, error) {
				res, errs := checkContainers(ctx, 0)

				select {
				case <-ctx.Done():
					return []types.ContainerJSON{}, fmt.Errorf("timeout context exceeded")
				case err = <-errs:
					return []types.ContainerJSON{}, fmt.Errorf("%v", err)
				case r := <-res:
					return r, nil
				}
			},
			timeout: 5 * time.Second,
		},
		{
			name: "get one container ID",
			run: func(ctx context.Context) ([]types.ContainerJSON, error) {

				err := startContainers(ctx, 1)
				if err != nil {
					return []types.ContainerJSON{}, err
				}

				res, errs := checkContainers(ctx, 1)

				select {
				case <-ctx.Done():
					return []types.ContainerJSON{}, fmt.Errorf("timeout context exceeded")
				case err = <-errs:
					return []types.ContainerJSON{}, fmt.Errorf("%v", err)
				case r := <-res:
					return r, nil
				}
			},
			timeout: 10 * time.Second,
		},
		{
			name: "get several container IDs",
			run: func(ctx context.Context) ([]types.ContainerJSON, error) {

				err := startContainers(ctx, 3)
				if err != nil {
					return []types.ContainerJSON{}, err
				}

				res, errs := checkContainers(ctx, 3)

				select {
				case <-ctx.Done():
					return []types.ContainerJSON{}, fmt.Errorf("timeout context exceeded")
				case err = <-errs:
					return []types.ContainerJSON{}, fmt.Errorf("%v", err)
				case r := <-res:
					return r, nil
				}
			},
			timeout: 30 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			newSpec.EndpointSpec.Ports[0].PublishMode = swarm.PortConfigPublishModeHost

			_, _, err := test.StartBrain(ctx, t, dockerClient, *newSpec)
			if err != nil {
				t.Errorf("%v", err)
				return
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, tt.timeout)
			defer cancel()
			defer test.DockerCleanUp(ctx, dockerClient, "")

			names, err := tt.run(timeoutCtx)
			if err != nil {
				t.Errorf("%v", err)
				return
			}

			got, err := stackContainerIDs(ctx, dockerClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("stackContainerIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, len(names), len(got))
			assert.Equal(t, names, got)
		})
	}
	if err := test.DockerCleanUp(ctx, dockerClient, ""); err != nil {
		t.Errorf("cleanup error: %v", err)
		return
	}
}

func TestNewLogMonitor(t *testing.T) {

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

	netID, err := test.CheckCreateNet("testnet")
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	newSpec := &swarm.ServiceSpec{}
	*newSpec = test.BrainSpec

	newSpec.Networks = []swarm.NetworkAttachmentConfig{
		swarm.NetworkAttachmentConfig{
			Target:  netID,
			Aliases: []string{"rethinkdb"},
		},
	}

	test.GenericPluginConfig.TaskTemplate.Networks = []swarm.NetworkAttachmentConfig{
		swarm.NetworkAttachmentConfig{
			Target: netID,
		},
	}

	tests := []struct {
		name    string
		run     func(context.Context) ([]types.ContainerJSON, error)
		timeout time.Duration
	}{
		{
			name: "test one container",
			run: func(ctx context.Context) ([]types.ContainerJSON, error) {
				err := startContainers(ctx, 1)
				if err != nil {
					return []types.ContainerJSON{}, err
				}

				res, errs := checkContainers(ctx, 1)

				select {
				case <-ctx.Done():
					return []types.ContainerJSON{}, fmt.Errorf("timeout context exceeded")
				case err = <-errs:
					return []types.ContainerJSON{}, fmt.Errorf("%v", err)
				case r := <-res:
					return r, nil
				}
			},
			timeout: 10 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newSpec.EndpointSpec.Ports[0].PublishMode = swarm.PortConfigPublishModeHost

			_, _, err := test.StartBrain(ctx, t, dockerClient, *newSpec)
			if err != nil {
				t.Errorf("%v", err)
				return
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, tt.timeout)
			defer cancel()
			defer test.DockerCleanUp(ctx, dockerClient, "")

			cons, errs := NewLogMonitor(timeoutCtx)

			res, err := tt.run(timeoutCtx)

			for {
				select {
				case <-timeoutCtx.Done():
					t.Errorf("timeout context exceeded")
					return
				case e := <-errs:
					t.Errorf("%v", e)
					return
				case c := <-cons:
					if c.Name == res[0].Name {
						assert.True(t, true)
						return
					}
				}
			}
		})
	}
	if err := test.DockerCleanUp(ctx, dockerClient, ""); err != nil {
		t.Errorf("cleanup error: %v", err)
		return
	}
}
