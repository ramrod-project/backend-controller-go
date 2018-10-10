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

	"github.com/ramrod-project/backend-controller-go/customtypes"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
)

func startTestContainers(ctx context.Context, number int) error {
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	newSpec := &swarm.ServiceSpec{}
	*newSpec = testServiceSpec
	newSpec.TaskTemplate.ContainerSpec.Command = []string{"/bin/sh", "-c", "while true; do echo 'test'; sleep 1; done"}

	for i := 0; i < number; i++ {
		newSpec.Name = "TestService" + strconv.Itoa(i)
		_, err = dockerClient.ServiceCreate(ctx, *newSpec, types.ServiceCreateOptions{})
		if err != nil {
			log.Printf("%v", err)
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

func checkServices(ctx context.Context, number int) (<-chan []swarm.Service, <-chan error) {
	ret := make(chan []swarm.Service)
	errs := make(chan error)
	services := make([]swarm.Service, number+1)

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
			svcs, err := dockerClient.ServiceList(ctx, types.ServiceListOptions{})
			if err != nil {
				errs <- err
				return
			}
			if len(svcs) == number+1 {
				for i := range services {
					if rethinkRegex.Match([]byte(svcs[i].Spec.Annotations.Name)) {
						continue
					}
					svc, _, err := dockerClient.ServiceInspectWithRaw(ctx, svcs[i].ID)
					if err != nil {
						errs <- err
						return
					}
					services[i] = svc
				}
				ret <- services
				return
			}
			time.Sleep(1000 * time.Millisecond)
		}
	}()
	return ret, errs
}

func Test_newLogger(t *testing.T) {

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

	networkID := ""

	tests := []struct {
		name    string
		run     func(context.Context) ([]swarm.Service, error)
		timeout time.Duration
	}{
		{
			name: "test 1",
			run: func(ctx context.Context) ([]swarm.Service, error) {
				err := startTestContainers(ctx, 1)
				if err != nil {
					return []swarm.Service{}, err
				}

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
			timeout: 20 * time.Second,
		},
		/*{
			name: "test actual",
			run: func(ctx context.Context) ([]swarm.Service, error) {
				dockerClient, err := client.NewEnvClient()
				if err != nil {
					return []swarm.Service{}, err
				}

				netID, err := test.CheckCreateNet("testnet")
				if err != nil {
					t.Errorf("%v", err)
					return []swarm.Service{}, err
				}
				networkID = netID

				newSpec := &swarm.ServiceSpec{}
				*newSpec = test.BrainSpec

				newSpec.Networks = []swarm.NetworkAttachmentConfig{
					swarm.NetworkAttachmentConfig{
						Target:  netID,
						Aliases: []string{"rethinkdb"},
					},
				}

				_, _, err = test.StartBrain(ctx, t, dockerClient, *newSpec)
				if err != nil {
					t.Errorf("%v", err)
					return []swarm.Service{}, err
				}

				newPluginSpec := &swarm.ServiceSpec{}
				*newPluginSpec = test.GenericPluginConfig

				newPluginSpec.Networks = []swarm.NetworkAttachmentConfig{
					swarm.NetworkAttachmentConfig{
						Target: netID,
					},
				}

				test.StartIntegrationTestService(ctx, dockerClient, *newPluginSpec)

				res, errs := checkServices(ctx, 1)

				for {
					select {
					case <-ctx.Done():
						return []swarm.Service{}, fmt.Errorf("timeout context exceeded")
					case err = <-errs:
						return []swarm.Service{}, fmt.Errorf("%v", err)
					case r := <-res:
						return r, nil
					}
				}
			},
			timeout: 20 * time.Second,
		},*/
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			timeoutCtx, cancel := context.WithTimeout(ctx, tt.timeout)
			defer cancel()
			defer test.DockerCleanUp(ctx, dockerClient, networkID)

			svcs, err := tt.run(timeoutCtx)
			if err != nil {
				t.Errorf("%v", err)
				return
			}

			out, errs := newLogger(timeoutCtx, dockerClient, svcs[0])
			nameMatch := regexp.MustCompile(svcs[0].Spec.Annotations.Name)

			for {
				select {
				case <-timeoutCtx.Done():
					t.Errorf("timeout context exceeded")
					return
				case e := <-errs:
					t.Errorf("%v", e)
					return
				case o := <-out:
					assert.True(t, nameMatch.Match([]byte(o.ServiceName)))
					assert.True(t, len(o.Log) > 0)
					return
				}
			}

		})
	}
	if err := test.DockerCleanUp(ctx, dockerClient, networkID); err != nil {
		t.Errorf("cleanup error: %v", err)
		return
	}
}

func TestNewLogHandler(t *testing.T) {

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

	networkID := ""
	brainServiceID := ""

	tests := []struct {
		name    string
		feed    func(context.Context, int) (<-chan swarm.Service, <-chan error)
		n       int
		timeout time.Duration
	}{
		{
			name: "test a few plugins",
			feed: func(ctx context.Context, number int) (<-chan swarm.Service, <-chan error) {
				ret := make(chan swarm.Service)
				errs := make(chan error)
				go func() {
					defer close(ret)
					defer close(errs)
					dockerClient, err := client.NewEnvClient()
					if err != nil {
						errs <- err
						return
					}

					netID, err := test.CheckCreateNet("testnet")
					if err != nil {
						errs <- fmt.Errorf("%v", err)
						return
					}

					networkID = netID

					newSpec := &swarm.ServiceSpec{}
					*newSpec = test.BrainSpec

					newSpec.Networks = []swarm.NetworkAttachmentConfig{
						swarm.NetworkAttachmentConfig{
							Target:  netID,
							Aliases: []string{"rethinkdb"},
						},
					}

					newPluginSpec := &swarm.ServiceSpec{}
					*newPluginSpec = test.GenericPluginConfig

					newPluginSpec.Networks = []swarm.NetworkAttachmentConfig{
						swarm.NetworkAttachmentConfig{
							Target: netID,
						},
					}

					_, brainID, err := test.StartBrain(ctx, t, dockerClient, *newSpec)
					if err != nil {
						errs <- fmt.Errorf("%v", err)
						return
					}
					brainServiceID = brainID

					for i := 0; i < number; i++ {
						newPluginSpec.Annotations.Name = newPluginSpec.Annotations.Name + strconv.Itoa(i)
						newPluginSpec.EndpointSpec.Ports[0].PublishedPort++
						svc, err := dockerClient.ServiceCreate(ctx, *newPluginSpec, types.ServiceCreateOptions{})
						if err != nil {
							errs <- err
							return
						}
						time.Sleep(500 * time.Millisecond)
						insp, _, err := dockerClient.ServiceInspectWithRaw(ctx, svc.ID)
						if err != nil {
							errs <- err
							return
						}
						ret <- insp
					}
				}()
				return ret, errs
			},
			n:       3,
			timeout: 30 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			timeoutCtx, cancel := context.WithTimeout(ctx, tt.timeout)
			defer cancel()
			defer test.DockerCleanUp(ctx, dockerClient, networkID)

			feedChan, feedErrs := tt.feed(timeoutCtx, tt.n)

			chanChan, logErrs := NewLogHandler(timeoutCtx, feedChan)

			// We should have n log chans total by the end
			chans := []<-chan customtypes.Log{}
		L:
			for {
				select {
				case <-timeoutCtx.Done():
					t.Errorf("timeout context exceeded")
					return
				case e, ok := <-feedErrs:
					if !ok {
						break
					}
					t.Errorf("%v", e)
					return
				case e := <-logErrs:
					t.Errorf("%v", e)
					return
				case c := <-chanChan:
					chans = append(chans, c)
					if len(chans) == tt.n {
						break L
					}
				}
			}

			for _, ch := range chans {
				count := 0
				for count < 4 {
					select {
					case <-timeoutCtx.Done():
						t.Errorf("timeout context exceeded")
						return
					case out := <-ch:
						log.Printf("log: %+v", out)
						assert.True(t, (len(out.Log) > 0))
						count++
					}
				}
			}

		})
	}
	test.KillService(ctx, dockerClient, brainServiceID)
	if err := test.DockerCleanUp(ctx, dockerClient, networkID); err != nil {
		t.Errorf("cleanup error: %v", err)
		return
	}
}
