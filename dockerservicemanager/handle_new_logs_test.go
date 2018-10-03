package dockerservicemanager

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
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

func checkContainerIDs(ctx context.Context, number int) (<-chan []types.ContainerJSON, <-chan error) {
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
					if rethinkRegex.Match([]byte(cons[i].Names[0])) {
						continue
					}
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

	networkID := ""

	tests := []struct {
		name    string
		run     func(context.Context) ([]types.ContainerJSON, error)
		timeout time.Duration
	}{
		{
			name: "test 1",
			run: func(ctx context.Context) ([]types.ContainerJSON, error) {
				err := startTestContainers(ctx, 1)
				if err != nil {
					return []types.ContainerJSON{}, err
				}

				res, errs := checkContainerIDs(ctx, 0)

				select {
				case <-ctx.Done():
					return []types.ContainerJSON{}, fmt.Errorf("timeout context exceeded")
				case err = <-errs:
					return []types.ContainerJSON{}, fmt.Errorf("%v", err)
				case r := <-res:
					return r, nil
				}
			},
			timeout: 20 * time.Second,
		},
		{
			name: "test actual",
			run: func(ctx context.Context) ([]types.ContainerJSON, error) {
				dockerClient, err := client.NewEnvClient()
				if err != nil {
					return []types.ContainerJSON{}, err
				}

				netID, err := test.CheckCreateNet("testnet")
				if err != nil {
					t.Errorf("%v", err)
					return []types.ContainerJSON{}, err
				}
				networkID = netID

				test.BrainSpec.Networks = []swarm.NetworkAttachmentConfig{
					swarm.NetworkAttachmentConfig{
						Target:  netID,
						Aliases: []string{"rethinkdb"},
					},
				}

				_, _, err = test.StartBrain(ctx, t, dockerClient, test.BrainSpec)
				if err != nil {
					t.Errorf("%v", err)
					return []types.ContainerJSON{}, err
				}

				test.GenericPluginConfig.Networks = []swarm.NetworkAttachmentConfig{
					swarm.NetworkAttachmentConfig{
						Target: netID,
					},
				}

				test.StartIntegrationTestService(ctx, dockerClient, test.GenericPluginConfig)

				res, errs := checkContainerIDs(ctx, 1)

				for {
					select {
					case <-ctx.Done():
						return []types.ContainerJSON{}, fmt.Errorf("timeout context exceeded")
					case err = <-errs:
						return []types.ContainerJSON{}, fmt.Errorf("%v", err)
					case r := <-res:
						return r, nil
					}
				}
			},
			timeout: 20 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			timeoutCtx, cancel := context.WithTimeout(ctx, tt.timeout)
			defer cancel()
			defer test.DockerCleanUp(ctx, dockerClient, networkID)

			cons, err := tt.run(timeoutCtx)
			if err != nil {
				t.Errorf("%v", err)
				return
			}

			out, errs := newContainerLogger(timeoutCtx, dockerClient, cons[0])
			nameMatch := regexp.MustCompile(strings.Split(strings.Split(cons[0].Name, "/")[1], ".")[0])

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
					assert.Equal(t, strings.Split(cons[0].Name, "/")[1], o.ContainerName)
					assert.Equal(t, cons[0].ID, o.ContainerID)
					assert.True(t, len(o.Log) > 0)
					return
				}
			}

		})
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

	tests := []struct {
		name    string
		feed    func(context.Context, int) (<-chan types.ContainerJSON, <-chan error)
		n       int
		timeout time.Duration
	}{
		{
			name: "test a few plugins",
			feed: func(ctx context.Context, number int) (<-chan types.ContainerJSON, <-chan error) {
				ret := make(chan types.ContainerJSON)
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

					test.BrainSpec.Networks = []swarm.NetworkAttachmentConfig{
						swarm.NetworkAttachmentConfig{
							Target:  netID,
							Aliases: []string{"rethinkdb"},
						},
					}

					test.GenericPluginConfig.Networks = []swarm.NetworkAttachmentConfig{
						swarm.NetworkAttachmentConfig{
							Target: netID,
						},
					}

					_, _, err = test.StartBrain(ctx, t, dockerClient, test.BrainSpec)
					if err != nil {
						errs <- fmt.Errorf("%v", err)
						return
					}

					services := make([]*regexp.Regexp, number)
					for i := 0; i < number; i++ {
						test.GenericPluginConfig.Annotations.Name = test.GenericPluginConfig.Annotations.Name + strconv.Itoa(i)
						test.GenericPluginConfig.EndpointSpec.Ports[0].PublishedPort++
						svc, err := dockerClient.ServiceCreate(ctx, test.GenericPluginConfig, types.ServiceCreateOptions{})
						if err != nil {
							errs <- err
							return
						}
						insp, _, err := dockerClient.ServiceInspectWithRaw(ctx, svc.ID)
						if err != nil {
							errs <- err
							return
						}
						services[i] = regexp.MustCompile(insp.Spec.Annotations.Name)
						time.Sleep(500 * time.Millisecond)
					}

					cons := []types.Container{}
					for {
						select {
						case <-ctx.Done():
							errs <- fmt.Errorf("timeout context exceeded")
							return
						default:
							break
						}
						cons, err = dockerClient.ContainerList(ctx, types.ContainerListOptions{})
						if err != nil {
							errs <- err
							return
						}
						if len(cons) == number+1 {
							break
						}
						time.Sleep(1000 * time.Millisecond)
					}

					for _, c := range cons {
						for _, r := range services {
							if r.Match([]byte(c.Names[0])) {
								res, err := dockerClient.ContainerInspect(ctx, c.ID)
								if err != nil {
									errs <- err
									return
								}
								ret <- res
							}
						}
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
			chans := []<-chan customtypes.ContainerLog{}
		L:
			for {
				select {
				case <-timeoutCtx.Done():
					t.Errorf("timeout context exceeded")
					return
				case e := <-feedErrs:
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
				for count < 3 {
					select {
					case <-timeoutCtx.Done():
						t.Errorf("timeout context exceeded")
						return
					case out := <-ch:
						assert.True(t, (len(out.Log) > 0))
						count++
					}
				}
			}

		})
	}
}
