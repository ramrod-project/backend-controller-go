package dockerservicemanager

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	swarm "github.com/docker/docker/api/types/swarm"
	rethink "github.com/ramrod-project/backend-controller-go/rethink"
)

var defaultEnvs = map[string]string{
	"LOGLEVEL": "DEBUG",
	"STAGE":    "DEV",
	"TAG":      "latest",
}

func envString(k string, v string) string {
	var stringBuf bytes.Buffer

	stringBuf.WriteString(k)
	stringBuf.WriteString("=")
	stringBuf.WriteString(v)

	return stringBuf.String()
}

func getEnvByKey(k string) string {
	var env string
	env = os.Getenv(k)

	if env == "" {
		env = defaultEnvs[k]
	}

	return envString(k, env)
}

func pluginToConfig(plugin rethink.Plugin) (PluginServiceConfig, error) {
	var (
		proto       swarm.PortConfigProtocol
		mode        swarm.PortConfigPublishMode
		environment []string
		extra       = false
	)

	mode = swarm.PortConfigPublishModeHost

	// Right now only one port is passed
	intPort, err := strconv.ParseUint(strings.Split(plugin.InternalPorts[0], "/")[0], 10, 32)
	if err != nil {
		return PluginServiceConfig{}, fmt.Errorf("Unable to parse port %v", plugin.InternalPorts[0])
	}
	extPort, err := strconv.ParseUint(strings.Split(plugin.ExternalPorts[0], "/")[0], 10, 32)
	if err != nil {
		return PluginServiceConfig{}, fmt.Errorf("Unable to parse port %v", plugin.ExternalPorts[0])
	}

	if plugin.Extra {
		extra = true
	}

	// Parse to check protocol
	if strings.Split(plugin.ExternalPorts[0], "/")[1] == string(swarm.PortConfigProtocolTCP) {
		proto = swarm.PortConfigProtocolTCP
	} else {
		proto = swarm.PortConfigProtocolUDP
	}

	// Concatonate environment variables (if provided)
	environment = []string{
		getEnvByKey("STAGE"),
		getEnvByKey("LOGLEVEL"),
		envString("PORT", fmt.Sprintf("%v", intPort)),
		envString("PLUGIN", plugin.Name),
		envString("PLUGIN_NAME", plugin.ServiceName),
	}
	if len(plugin.Environment) > 0 {
		environment = append(environment, plugin.Environment...)
	}

	return PluginServiceConfig{
		Extra:       extra,
		Environment: environment,
		Address:     plugin.Address,
		Network:     "pcp",
		OS:          plugin.OS,
		Ports: []swarm.PortConfig{swarm.PortConfig{
			Protocol:      proto,
			TargetPort:    uint32(intPort),
			PublishedPort: uint32(extPort),
			PublishMode:   mode,
		}},
		ServiceName: plugin.ServiceName,
	}, nil
}

func selectChange(plugin rethink.Plugin) error {
	// if plugin has no servicename, it cannot be started
	if plugin.ServiceName == "" {
		return nil
	}
	switch plugin.DesiredState {
	case rethink.DesiredStateActivate:
		config, err := pluginToConfig(plugin)
		if err != nil {
			return err
		}
		_, err = CreatePluginService(&config)
		return err
	case rethink.DesiredStateRestart:
		config, err := pluginToConfig(plugin)
		if err != nil {
			return err
		}
		_, err = UpdatePluginService(plugin.ServiceID, &config)
		return err
	case rethink.DesiredStateStop:
		err := RemovePluginService(plugin.ServiceID)
		return err
	case rethink.DesiredStateNull:
		return nil
	}
	return fmt.Errorf("desired state not matched")
}

// HandlePluginChanges takes a channel of Plugins
// being fed by the plugin monitor routine and performs
// actions on their services as needed.
func HandlePluginChanges(feed <-chan rethink.Plugin) <-chan error {
	errChan := make(chan error)

	go func(in <-chan rethink.Plugin) {
		for plugin := range in {
			err := selectChange(plugin)
			if err != nil {
				errChan <- err
			}
		}
	}(feed)

	return errChan
}
