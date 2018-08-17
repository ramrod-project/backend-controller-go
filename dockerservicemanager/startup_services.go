package dockerservicemanager

import (
	"os"
	"strings"

	mount "github.com/docker/docker/api/types/mount"
	swarm "github.com/docker/docker/api/types/swarm"

	"github.com/ramrod-project/backend-controller-go/rethink"
)

var harnessConfig = PluginServiceConfig{
	Environment: []string{
		strings.Replace(getEnvByKey("STAGE"), "TESTING", "DEV", 1),
		getEnvByKey("LOGLEVEL"),
		envString("PORT", "5000"),
		envString("PLUGIN", "Harness"),
	},
	Address: "",
	Network: "pcp",
	OS:      rethink.PluginOSAll,
	Ports: []swarm.PortConfig{
		swarm.PortConfig{
			Protocol:      swarm.PortConfigProtocolTCP,
			TargetPort:    uint32(5000),
			PublishedPort: uint32(5000),
			PublishMode:   swarm.PortConfigPublishModeIngress,
		},
	},
	ServiceName: "Harness-5000",
}

var auxConfig = PluginServiceConfig{
	Environment: []string{
		strings.Replace(getEnvByKey("STAGE"), "TESTING", "DEV", 1),
		getEnvByKey("LOGLEVEL"),
		getEnvByKey("TAG"),
	},
	Address:     "",
	Network:     "pcp",
	OS:          rethink.PluginOSAll,
	ServiceName: "AuxiliaryServices",
	Volumes: []mount.Mount{
		mount.Mount{
			Type:   mount.TypeBind,
			Source: "/var/run/docker.sock",
			Target: "/var/run/docker.sock",
		},
	},
}

// StartupServices will start the Aux Services Service
// and a Harness plugin service if the HARNESS_START
// and AUX_START environment variables are set to YES.
func StartupServices() error {

	if os.Getenv("START_HARNESS") == "YES" {
		res, err := CreatePluginService(&harnessConfig)
		if err != nil {
			return err
		}
		err = advertiseStartupService(map[string]interface{}{
			"Name":          "Harness",
			"ServiceID":     res.ID,
			"ServiceName":   harnessConfig.ServiceName,
			"DesiredState":  "",
			"State":         "Active",
			"Interface":     "",
			"ExternalPorts": []string{"5000/tcp"},
			"InternalPorts": []string{"5000/tcp"},
			"OS":            string(rethink.PluginOSAll),
			"Environment":   []string{},
		})
		if err != nil {
			return err
		}
	}

	if os.Getenv("START_AUX") == "YES" {
		res, err := CreatePluginService(&auxConfig)
		if err != nil {
			return err
		}
		err = advertiseStartupService(map[string]interface{}{
			"Name":          "AuxServices",
			"ServiceID":     res.ID,
			"ServiceName":   auxConfig.ServiceName,
			"DesiredState":  "",
			"State":         "Active",
			"Interface":     "",
			"ExternalPorts": []string{"20/tcp", "21/tcp", "80/tcp", "53/udp"},
			"InternalPorts": []string{"20/tcp", "21/tcp", "80/tcp", "53/udp"},
			"OS":            string(rethink.PluginOSPosix),
			"Environment":   []string{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}
