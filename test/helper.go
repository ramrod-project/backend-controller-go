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

func getTagFromEnv() string {
	temp := os.Getenv("TAG")
	if temp == "" {
		temp = os.Getenv("TRAVIS_BRANCH")
	}
	if temp == "" {
		temp = "latest"
	}
	return temp
}

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

func DockerCleanUp(ctx context.Context, dockerClient *client.Client, net string) error {
	//Docker cleanup
	services, err := dockerClient.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	for _, v := range services {
		if v.ID == "" {
			continue
		}
		err := dockerClient.ServiceRemove(ctx, v.ID)
		if err != nil {
			return fmt.Errorf("%v", err)
		}
	}
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
	for _, c := range containers {
		if c.ID == "" {
			continue
		}
		err := dockerClient.ContainerKill(ctx, c.ID, "SIGKILL")
		if err != nil {
			return fmt.Errorf("%v", err)
		}
		err = dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			return fmt.Errorf("%v", err)
		}
	}
	start := time.Now()
	for time.Since(start) < 10*time.Second {
		dockerClient.NetworkRemove(ctx, net)
		time.Sleep(time.Second)
		_, err := dockerClient.NetworkInspect(ctx, net)
		if err != nil {
			_, err := dockerClient.NetworksPrune(ctx, filters.Args{})
			if err != nil {
				break
			}
		}
	}
	return nil
}

func GetImage(image string) string {
	var stringBuf bytes.Buffer

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = os.Getenv("TRAVIS_BRANCH")
	}
	if tag == "" {
		tag = "latest"
	}

	stringBuf.WriteString(image)
	stringBuf.WriteString(":")
	stringBuf.WriteString(tag)

	return stringBuf.String()
}

func StartBrain(ctx context.Context, t *testing.T, dockerClient *client.Client, spec swarm.ServiceSpec) (*r.Session, string, error) {
	// Start brain
	result, err := dockerClient.ServiceCreate(ctx, spec, types.ServiceCreateOptions{})
	if err != nil {
		t.Errorf("%v", err)
		return nil, "", err
	}

	// Test setup
	session, err := r.Connect(r.ConnectOpts{
		Address: "127.0.0.1",
	})
	start := time.Now()
	if err != nil {
		for {
			if time.Since(start) >= 20*time.Second {
				t.Errorf("%v", err)
				return nil, "", err
			}
			session, err = r.Connect(r.ConnectOpts{
				Address: "127.0.0.1",
			})
			if err == nil {
				_, err := r.DB("Controller").Table("Plugins").Run(session)
				if err == nil {
					break
				}
			}
			time.Sleep(time.Second)
		}
	}
	return session, result.ID, nil
}

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
	},
	TaskTemplate: swarm.TaskSpec{
		ContainerSpec: swarm.ContainerSpec{
			DNSConfig: &swarm.DNSConfig{},
			Env:       []string{"TAG=" + getTagFromEnv()},
			Image:     GetImage("ramrodpcp/backend-controller"),
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
