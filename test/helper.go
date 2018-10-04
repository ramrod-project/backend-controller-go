package test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	mount "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	r "gopkg.in/gorethink/gorethink.v4"
)

var GenericPluginConfig = swarm.ServiceSpec{
	Annotations: swarm.Annotations{
		Name: "Harness-5000tcp",
		Labels: map[string]string{
			"os": "posix",
		},
	},
	TaskTemplate: swarm.TaskSpec{
		ContainerSpec: swarm.ContainerSpec{
			Env: []string{
				"PLUGIN=Harness",
				"PORT=5000",
				"LOGLEVEL=DEBUG",
				"STAGE=DEV",
				"PLUGIN_NAME=Harness-5000tcp",
			},
			Image: "ramrodpcp/interpreter-plugin:" + getTagFromEnv(),
		},
		RestartPolicy: &swarm.RestartPolicy{
			Condition: "on-failure",
		},
	},
	UpdateConfig: &swarm.UpdateConfig{
		Parallelism: 0,
		Delay:       0,
	},
	EndpointSpec: &swarm.EndpointSpec{
		Mode: swarm.ResolutionModeVIP,
		Ports: []swarm.PortConfig{
			swarm.PortConfig{
				Protocol:      swarm.PortConfigProtocolTCP,
				PublishedPort: 5000,
				TargetPort:    5000,
				PublishMode:   swarm.PortConfigPublishModeHost,
			},
		},
	},
}

func getTagFromEnv() string {
	temp := os.Getenv("TAG")
	if temp == "" {
		temp = "latest"
	}
	return temp
}

// GetServiceID Gets the service ID of a service given the name of the service
func GetServiceID(ctx context.Context, dockerClient *client.Client, name string) string {
	services, err := dockerClient.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return ""
	}
	for _, service := range services {
		if service.Spec.Annotations.Name == name {
			return service.ID
		}
	}
	return ""
}

// DockerCleanUp removes all services and containers
func DockerCleanUp(ctx context.Context, dockerClient *client.Client, net string) error {
	// Timeout
	timeoutContext, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// Remove Services
	delSvc := func() <-chan struct{} {
		res := make(chan struct{})
		go func() {
			for {
				select {
				case <-timeoutContext.Done():
					close(res)
					return
				default:
					break
				}
				time.Sleep(100 * time.Millisecond)
				services, err := dockerClient.ServiceList(timeoutContext, types.ServiceListOptions{})
				if len(services) == 0 {
					res <- struct{}{}
					return
				}
				if err != nil {
					continue
				}
				for _, v := range services {
					dockerClient.ServiceRemove(ctx, v.ID)
				}
			}
		}()
		return res
	}()

	// Wait for no services
	select {
	case <-timeoutContext.Done():
		return fmt.Errorf("services not killed in timeout")
	case <-delSvc:
		break
	}

	// Remove Containers
	delCon := func() <-chan struct{} {
		res := make(chan struct{})
		go func() {
			for {
				select {
				case <-timeoutContext.Done():
					close(res)
					return
				default:
					break
				}
				time.Sleep(100 * time.Millisecond)
				containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
				if len(containers) == 0 {
					res <- struct{}{}
					return
				}
				if err != nil {
					continue
				}
				for _, c := range containers {
					dockerClient.ContainerKill(ctx, c.ID, "SIGKILL")
					dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})
				}
			}
		}()
		return res
	}()

	// Wait for no containers
	select {
	case <-timeoutContext.Done():
		return fmt.Errorf("containers not killed in timeout")
	case <-delCon:
		break
	}

	if net != "" {
		delNet := func() <-chan struct{} {
			res := make(chan struct{})
			go func() {
			L:
				for {
					select {
					case <-timeoutContext.Done():
						close(res)
						return
					default:
						break
					}
					dockerClient.NetworksPrune(ctx, filters.Args{})
					time.Sleep(500 * time.Millisecond)
					netList, err := dockerClient.NetworkList(ctx, types.NetworkListOptions{})
					if err != nil {
						continue L
					}
					for _, n := range netList {
						if n.ID == net {
							continue L
						}
					}
					res <- struct{}{}
					return
				}
			}()
			return res
		}()

		// Wait for network removal
		select {
		case <-timeoutContext.Done():
			return fmt.Errorf("network not removed")
		case <-delNet:
			break
		}
	}
	return nil
}

// GetImage gets the image name with its tag
func GetImage(image string) string {
	var stringBuf bytes.Buffer

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = "latest"
	}

	stringBuf.WriteString(image)
	stringBuf.WriteString(":")
	stringBuf.WriteString(tag)

	return stringBuf.String()
}

// CheckCreateNet checks to see if the given network is created and creates
// it if it is not
func CheckCreateNet(net string) (string, error) {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return "", err
	}

	nets, err := dockerClient.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return "", err
	}
	for _, n := range nets {
		if n.Name == net {
			return n.ID, nil
		}
	}

	netID, err := dockerClient.NetworkCreate(ctx, net, types.NetworkCreate{
		Driver:     "overlay",
		Attachable: true,
	})
	if err != nil {
		return "", err
	}
	return netID.ID, nil
}

// StartBrain starts an instance of the Brain module and returns a connection
// (*Session) to it
func StartBrain(ctx context.Context, t *testing.T, dockerClient *client.Client, spec swarm.ServiceSpec) (*r.Session, string, error) {
	// Timeout
	timeoutContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Start brain
	var resID string
	startB := func() <-chan string {
		res := make(chan string)
		go func() {
			for {
				time.Sleep(1000 * time.Millisecond)
				result, err := dockerClient.ServiceCreate(timeoutContext, spec, types.ServiceCreateOptions{})
				if err != nil {
					if result.ID != "" {
						KillService(timeoutContext, dockerClient, result.ID)
					}
					continue
				}
				res <- result.ID
			}
		}()
		return res
	}()

	// Wait for successful start
	select {
	case <-timeoutContext.Done():
		return nil, "", fmt.Errorf("brain no started in timeout")
	case resID = <-startB:
		if resID == "" {
			return nil, "", fmt.Errorf("no ID received for brain")
		}
		break
	}

	// Test brain connection
	testB := func() <-chan struct{} {
		res := make(chan struct{})
		go func() {
			for {
				time.Sleep(500 * time.Millisecond)
				session, err := r.Connect(r.ConnectOpts{
					Address: "127.0.0.1",
				})
				if err != nil {
					continue
				}
				_, err = r.DB("Controller").Table("Plugins").Run(session)
				if err != nil {
					continue
				}
				res <- struct{}{}
			}
		}()
		return res
	}()

	// Wait for successful connection
	select {
	case <-timeoutContext.Done():
		return nil, "", fmt.Errorf("brain connection not established in timeout")
	case <-testB:
		sessionRet, err := r.Connect(r.ConnectOpts{
			Address: "127.0.0.1",
		})
		return sessionRet, resID, err
	}
}

// KillService Kills a docker service
func KillService(ctx context.Context, dockerClient *client.Client, svcID string) {
	start := time.Now()
	for time.Since(start) < 10*time.Second {
		err := dockerClient.ServiceRemove(ctx, svcID)
		if err != nil {
			break
		}
		time.Sleep(time.Second)
	}
	for time.Since(start) < 15*time.Second {
		containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
		if err == nil {
			if len(containers) == 0 {
				break
			}
			for _, c := range containers {
				err = dockerClient.ContainerKill(ctx, c.ID, "")
				if err == nil {
					dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})
				}
			}
		}
		time.Sleep(time.Second)
	}
}

// StartIntegrationTestService starts a service for the integration test
func StartIntegrationTestService(ctx context.Context, dockerClient *client.Client, spec swarm.ServiceSpec) (string, error) {

	// Start service
	result, err := dockerClient.ServiceCreate(ctx, spec, types.ServiceCreateOptions{})
	if err != nil {
		return "", err
	}
	return result.ID, nil
}

var BrainSpec = swarm.ServiceSpec{
	Annotations: swarm.Annotations{
		Name: "rethinkdb",
	},
	TaskTemplate: swarm.TaskSpec{
		ContainerSpec: swarm.ContainerSpec{
			DNSConfig: &swarm.DNSConfig{},
			Image:     GetImage("ramrodpcp/database-brain"),
		},
		RestartPolicy: &swarm.RestartPolicy{
			Condition: "on-failure",
		},
	},
	EndpointSpec: &swarm.EndpointSpec{
		Mode: swarm.ResolutionModeVIP,
		Ports: []swarm.PortConfig{swarm.PortConfig{
			Protocol:      swarm.PortConfigProtocolTCP,
			TargetPort:    28015,
			PublishedPort: 28015,
			PublishMode:   swarm.PortConfigPublishModeIngress,
		}},
	},
}

var controllerSpec = swarm.ServiceSpec{
	Annotations: swarm.Annotations{
		Name: "controller",
		Labels: map[string]string{
			"com.docker.stack.namespace": "test",
		},
	},
	TaskTemplate: swarm.TaskSpec{
		ContainerSpec: swarm.ContainerSpec{
			DNSConfig: &swarm.DNSConfig{},
			Env:       []string{"TAG=" + getTagFromEnv()},
			Image:     "ramrodpcp/backend-controller:test",
			Mounts: []mount.Mount{
				mount.Mount{
					Type:   mount.TypeBind,
					Source: "/var/run/docker.sock",
					Target: "/var/run/docker.sock",
				},
			},
		},
		Networks: []swarm.NetworkAttachmentConfig{
			swarm.NetworkAttachmentConfig{
				Target: "pcp",
			},
		},
		RestartPolicy: &swarm.RestartPolicy{
			Condition: "on-failure",
		},
	},
	EndpointSpec: &swarm.EndpointSpec{
		Mode: swarm.ResolutionModeVIP,
	},
}
