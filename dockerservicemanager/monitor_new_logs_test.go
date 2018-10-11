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
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
)

func startServices(ctx context.Context, number int) error {
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

func Test_stackServices(t *testing.T) {

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
		run     func(context.Context) ([]swarm.Service, error)
		timeout time.Duration
		wantErr bool
	}{
		{
			name: "get nothing",
			run: func(ctx context.Context) ([]swarm.Service, error) {
				res, errs := checkServices(ctx, 0)

				select {
				case <-ctx.Done():
					return []swarm.Service{}, fmt.Errorf("timeout context exceeded")
				case err = <-errs:
					return []swarm.Service{}, fmt.Errorf("%v", err)
				case r := <-res:
					return r, nil
				}
			},
			timeout: 5 * time.Second,
		},
		{
			name: "get one service",
			run: func(ctx context.Context) ([]swarm.Service, error) {

				err := startServices(ctx, 1)
				if err != nil {
					return []swarm.Service{}, err
				}

				res, errs := checkServices(ctx, 1)

				select {
				case <-ctx.Done():
					return []swarm.Service{}, fmt.Errorf("timeout context exceeded")
				case err = <-errs:
					return []swarm.Service{}, fmt.Errorf("%v", err)
				case r := <-res:
					return r, nil
				}
			},
			timeout: 10 * time.Second,
		},
		{
			name: "get several services",
			run: func(ctx context.Context) ([]swarm.Service, error) {

				err := startServices(ctx, 3)
				if err != nil {
					return []swarm.Service{}, err
				}

				res, errs := checkServices(ctx, 3)

				select {
				case <-ctx.Done():
					return []swarm.Service{}, fmt.Errorf("timeout context exceeded")
				case err = <-errs:
					return []swarm.Service{}, fmt.Errorf("%v", err)
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

			got, err := stackServices(ctx, dockerClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("stackServices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, len(names), len(got))
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
		run     func(context.Context) ([]swarm.Service, error)
		timeout time.Duration
	}{
		{
			name: "test one service",
			run: func(ctx context.Context) ([]swarm.Service, error) {
				err := startServices(ctx, 1)
				if err != nil {
					return []swarm.Service{}, err
				}

				res, errs := checkServices(ctx, 1)

				select {
				case <-ctx.Done():
					return []swarm.Service{}, fmt.Errorf("timeout context exceeded")
				case err = <-errs:
					return []swarm.Service{}, fmt.Errorf("%v", err)
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

			svcs, errs := NewLogMonitor(timeoutCtx)

			res, err := tt.run(timeoutCtx)

			for {
				select {
				case <-timeoutCtx.Done():
					t.Errorf("timeout context exceeded")
					return
				case e := <-errs:
					t.Errorf("%v", e)
					return
				case s := <-svcs:
					for _, svc := range res {
						if s.Spec.Annotations.Name == svc.Spec.Annotations.Name {
							assert.True(t, true)
							return
						}
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
