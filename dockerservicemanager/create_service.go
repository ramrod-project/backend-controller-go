package dockerservicemanager

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	types "github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	mount "github.com/docker/docker/api/types/mount"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	rethink "github.com/ramrod-project/backend-controller-go/rethink"
)

type dockerImageName struct {
	Name string
	Tag  string
}

// String method for image name
func (d dockerImageName) String() string {
	var stringBuf bytes.Buffer

	stringBuf.WriteString(d.Name)
	stringBuf.WriteString(":")
	stringBuf.WriteString(d.Tag)

	return stringBuf.String()
}

// PluginServiceConfig contains configuration parameters
// for a plugin service.
type PluginServiceConfig struct {
	Environment []string
	Address     string
	Network     string
	OS          rethink.PluginOS
	Ports       []swarm.PortConfig `json:",omitempty"`
	ServiceName string
	Volumes     []mount.Mount `json:",omitempty"`
}

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

func hostString(h string, i string) string {
	var stringBuf bytes.Buffer

	stringBuf.WriteString(h)
	stringBuf.WriteString(":")
	stringBuf.WriteString(i)

	return stringBuf.String()
}

func getManagerIP() string {
	ctx := context.TODO()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	list, err := dockerClient.NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		panic(err)
	}

	for _, node := range list {
		if node.Spec.Role == "manager" {
			return node.Status.Addr
		}
	}
	return ""
}

func generateServiceSpec(config *PluginServiceConfig) (*swarm.ServiceSpec, error) {
	var (
		hosts     []string
		imageName = &dockerImageName{
			Tag: getTagFromEnv(),
		}
		labels          = make(map[string]string)
		maxAttempts     = uint64(3)
		placementConfig = &swarm.Placement{}
		replicas        = uint64(1)
		stopGrace       = time.Second
	)

	log.Printf("Creating service %v with config %v\n", config.ServiceName, config)

	// Determine container image
	if config.OS == rethink.PluginOSPosix || config.OS == rethink.PluginOSAll {
		imageName.Name = "ramrodpcp/interpreter-plugin"
	} else if config.OS == rethink.PluginOSWindows {
		imageName.Name = "ramrodpcp/interpreter-plugin-windows"
		placementConfig.Constraints = []string{"node.labels.os==nt"}
		hosts = append(hosts, hostString("rethinkdb", getManagerIP()))
	} else {
		return &swarm.ServiceSpec{}, fmt.Errorf("invalid OS setting: %v", config.OS)
	}

	// Check if IP specified
	if config.Address != "" {
		placementConfig.Constraints = append(placementConfig.Constraints, "node.labels.ip=="+config.Address)
	}

	log.Printf("Creating service spec\n")

	serviceSpec := &swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: config.ServiceName,
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				DNSConfig: &swarm.DNSConfig{},
				Env:       config.Environment,
				Healthcheck: &container.HealthConfig{
					Interval: time.Second,
					Timeout:  time.Second * 3,
					Retries:  3,
				},
				Image:           imageName.String(),
				Labels:          labels,
				Mounts:          config.Volumes,
				OpenStdin:       false,
				StopGracePeriod: &stopGrace,
				TTY:             false,
				Hosts:           hosts,
			},
			RestartPolicy: &swarm.RestartPolicy{
				Condition:   "on-failure",
				MaxAttempts: &maxAttempts,
			},
			Placement: placementConfig,
			Networks: []swarm.NetworkAttachmentConfig{
				swarm.NetworkAttachmentConfig{
					Target: config.Network,
				},
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicas,
			},
		},
		UpdateConfig: &swarm.UpdateConfig{
			Parallelism: 0,
			Delay:       0,
		},
		EndpointSpec: &swarm.EndpointSpec{
			Mode:  swarm.ResolutionModeVIP,
			Ports: config.Ports,
		},
	}

	return serviceSpec, nil
}

// CreatePluginService creates a service for a plugin
// given a PluginServiceConfig.
func CreatePluginService(config *PluginServiceConfig) (types.ServiceCreateResponse, error) {
	ctx := context.TODO()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		return types.ServiceCreateResponse{}, err
	}

	log.Printf("Generating service spec\n")

	serviceSpec, err := generateServiceSpec(config)

	if err != nil {
		return types.ServiceCreateResponse{}, err
	}

	log.Printf("Service spec created: %v", serviceSpec)

	resp, err := dockerClient.ServiceCreate(ctx, *serviceSpec, types.ServiceCreateOptions{})
	return resp, err

}
