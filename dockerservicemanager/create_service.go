package dockerservicemanager

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
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

func GetManagerIP() string {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	list, err := dockerClient.NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		panic(err)
	}

	for _, node := range list {
		if node.Spec.Role == swarm.NodeRoleManager {
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

	// Determine container image
	if config.ServiceName == "AuxiliaryServices" {
		imageName.Name = "ramrodpcp/auxiliary-wrapper"
	} else if config.OS == rethink.PluginOSPosix || config.OS == rethink.PluginOSAll {
		imageName.Name = "ramrodpcp/interpreter-plugin"
		placementConfig.Constraints = []string{"node.labels.os==posix"}
	} else if config.OS == rethink.PluginOSWindows {
		imageName.Name = "ramrodpcp/interpreter-plugin-windows"
		placementConfig.Constraints = []string{"node.labels.os==nt"}
		hosts = append(hosts, hostString("rethinkdb", GetManagerIP()))
		config.Environment = append(config.Environment, "RETHINK_HOST="+GetManagerIP())
	} else {
		return &swarm.ServiceSpec{}, fmt.Errorf("invalid OS setting: %v", config.OS)
	}

	// Check if IP specified
	if config.Address != "" {
		placementConfig.Constraints = append(placementConfig.Constraints, "node.labels.ip=="+config.Address)
	}

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

	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		return types.ServiceCreateResponse{}, err
	}

	serviceSpec, err := generateServiceSpec(config)

	if err != nil {
		return types.ServiceCreateResponse{}, err
	}

	resp, err := dockerClient.ServiceCreate(ctx, *serviceSpec, types.ServiceCreateOptions{})
	log.Printf("Started service %+v", resp)

	//update ports
	for _, port := range config.Ports {
		err := rethink.AddPort(config.Address, strconv.FormatUint(uint64(port.PublishedPort), 10), port.Protocol)
		if err != nil {
			log.Printf("%v", err)
		}
	}

	return resp, err

}
