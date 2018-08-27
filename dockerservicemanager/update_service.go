package dockerservicemanager

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"time"

	"github.com/ramrod-project/backend-controller-go/rethink"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
)

func checkReady(ctx context.Context, dockerClient *client.Client, serviceID string) (uint64, error) {

	start := time.Now()
	for {
		if time.Since(start) > 10*time.Second {
			break
		}
		inspectResults, _, err := dockerClient.ServiceInspectWithRaw(ctx, serviceID)
		if err != nil {
			return 0, err
		}
		if inspectResults.UpdateStatus.State != swarm.UpdateStateUpdating {
			return inspectResults.Version.Index, nil
		}
		time.Sleep(time.Second)
	}
	return 0, fmt.Errorf("timeout: service %v still updating", serviceID)
}

func containsPort(port *swarm.PortConfig, comparePorts *[]swarm.PortConfig) bool {
	for _, cP := range *comparePorts {
		if reflect.DeepEqual(*port, cP) {
			return true
		}
	}
	return false
}

// UpdatePluginService updates a given service by ID string
// and given a valid PluginServiceConfig. It will attempt
// to update the service and relaunch it.
func UpdatePluginService(serviceID string, config *PluginServiceConfig) (types.ServiceUpdateResponse, error) {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()

	if err != nil {
		return types.ServiceUpdateResponse{}, err
	}

	serviceSpec, err := generateServiceSpec(config)
	if err != nil {
		return types.ServiceUpdateResponse{}, err
	}

	version, err := checkReady(ctx, dockerClient, serviceID)
	if err != nil {
		return types.ServiceUpdateResponse{}, err
	}

	serv, _, err := dockerClient.ServiceInspectWithRaw(ctx, serviceID)
	if err != nil {
		log.Printf("%v", err)
	}
	resp, err := dockerClient.ServiceUpdate(ctx, serviceID, swarm.Version{Index: version}, *serviceSpec, types.ServiceUpdateOptions{})
	if err != nil {
		return resp, err
	}
	for _, port := range serv.Spec.EndpointSpec.Ports {
		// if old port is not in new ports
		if !containsPort(&port, &config.Ports) {
			err = rethink.RemovePort(config.Address, strconv.FormatUint(uint64(port.PublishedPort), 10), port.Protocol)
			if err != nil {
				log.Printf("%v", err)
			}
		}
	}
	for _, port := range config.Ports {
		err = rethink.AddPort(config.Address, strconv.FormatUint(uint64(port.PublishedPort), 10), port.Protocol)
		if err != nil {
			log.Printf("%v", err)
		}
	}

	return resp, err
}
