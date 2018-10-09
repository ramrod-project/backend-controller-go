package dockerservicemanager

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	types "github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	mount "github.com/docker/docker/api/types/mount"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	rethink "github.com/ramrod-project/backend-controller-go/rethink"
)

var ipv4 = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)

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
	Extra       bool
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

// GetManagerIP returns the string version of the primary IPv4
// address associated with the manager node in the swarm.
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
		annotations = swarm.Annotations{
			Name:   config.ServiceName,
			Labels: make(map[string]string),
		}
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
		annotations.Labels["os"] = "posix"
		placementConfig.Constraints = []string{"node.labels.os==posix"}
		imageName.Name = "ramrodpcp/auxiliary-wrapper"
	} else if config.OS == rethink.PluginOSPosix || config.OS == rethink.PluginOSAll {
		annotations.Labels["os"] = "posix"
		if config.Extra {
			imageName.Name = "ramrodpcp/interpreter-plugin-extra"
		} else {
			imageName.Name = "ramrodpcp/interpreter-plugin"
		}
		placementConfig.Constraints = []string{"node.labels.os==posix"}
		hosts = append(hosts, hostString("rethinkdb", GetManagerIP()))
		config.Environment = append(config.Environment, "RETHINK_HOST="+GetManagerIP())
	} else if config.OS == rethink.PluginOSWindows {
		annotations.Labels["os"] = "nt"
		imageName.Name = "ramrodpcp/interpreter-plugin-windows"
		placementConfig.Constraints = []string{"node.labels.os==nt"}
		hosts = append(hosts, hostString("rethinkdb", GetManagerIP()))
		config.Environment = append(config.Environment, "RETHINK_HOST="+GetManagerIP())
	} else {
		return &swarm.ServiceSpec{}, fmt.Errorf("invalid OS setting: %v", config.OS)
	}

	// Check if IP specified and valid
	if v := config.Address; v != "" && ipv4.MatchString(v) {
		var stringBuf bytes.Buffer
		stringBuf.WriteString("node.labels.ip==")
		stringBuf.WriteString(v)
		placementConfig.Constraints = append(placementConfig.Constraints, stringBuf.String())
	} else {
		return &swarm.ServiceSpec{}, fmt.Errorf("must specify valid ip address, got: %v", config.Address)
	}

	serviceSpec := &swarm.ServiceSpec{
		Annotations: annotations,
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
