package dockerservicemanager

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	rethink "github.com/ramrod-project/backend-controller-go/rethink"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
)

func validateService(config rethink.Plugin, result swarm.Service, netID string) error {
	if result.ID == "" {
		return fmt.Errorf("no service ID found")
	} else if result.Spec.Annotations.Name != config.ServiceName {
		return fmt.Errorf("servicename %v doesn't match config ServiceName %v", result.Spec.Annotations.Name, config.ServiceName)
	} else if result.Spec.TaskTemplate.ContainerSpec.Image != "ramrodpcp/interpreter-plugin:"+getTagFromEnv() {
		return fmt.Errorf("imagename %v doesn't match expected", result.Spec.TaskTemplate.ContainerSpec.Image)
	} else if len(result.Spec.TaskTemplate.Networks) != 1 || result.Spec.TaskTemplate.Networks[0].Target != netID {
		return fmt.Errorf("network %v doesn't match %v(pcp)", result.Spec.TaskTemplate.Networks[0].Target, netID)
	}
	if config.Address != "" {
		ipFound := false
		address := ""
		for _, c := range result.Spec.TaskTemplate.Placement.Constraints {
			address = strings.Split(c, "==")[1]
			if address == config.Address {
				ipFound = true
				break
			}
		}
		if !ipFound {
			return fmt.Errorf("address %v doesn't match %v", address, config.Address)
		}
	}
	for _, i := range config.Environment {
		found := false
		for _, j := range result.Spec.TaskTemplate.ContainerSpec.Env {
			if i == j {
				found = true
				break
			}
		}
		if found == false {
			return fmt.Errorf("environment %v not found in service", i)
		}
	}
	return nil
}

func Test_envString(t *testing.T) {
	type args struct {
		k string
		v string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Basi key value",
			args: args{
				k: "KEY",
				v: "VALUE",
			},
			want: "KEY=VALUE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := envString(tt.args.k, tt.args.v); got != tt.want {
				t.Errorf("envString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getEnvByKey(t *testing.T) {

	tests := []struct {
		name string
		set  string
		key  string
		want string
	}{
		{
			name: "Test stage good",
			set:  "TEST",
			key:  "STAGE",
			want: "STAGE=TEST",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldEnv := os.Getenv(tt.key)
			os.Setenv(tt.key, tt.set)
			if got := getEnvByKey(tt.key); got != tt.want {
				t.Errorf("getEnvByKey() = %v, want %v", got, tt.want)
			}
			os.Setenv(tt.key, oldEnv)
		})
	}
}

func Test_pluginToConfig(t *testing.T) {
	stage := os.Getenv("STAGE")
	if stage == "" {
		stage = "DEV"
	}
	log := os.Getenv("LOGLEVEL")
	if log == "" {
		log = "DEBUG"
	}

	type args struct {
		plugin rethink.Plugin
	}
	tests := []struct {
		name    string
		args    args
		want    PluginServiceConfig
		wantErr bool
	}{
		{
			name: "Basic plugin",
			args: args{
				plugin: rethink.Plugin{
					Name:          "BasicPlugin",
					ServiceID:     "",
					ServiceName:   "BasicPluginService",
					DesiredState:  "",
					State:         "Available",
					Address:       "192.168.1.1",
					ExternalPorts: []string{"5000/tcp"},
					InternalPorts: []string{"5000/tcp"},
					OS:            rethink.PluginOSAll,
					Environment:   []string{},
				},
			},
			want: PluginServiceConfig{
				Environment: []string{
					"STAGE=" + stage,
					"LOGLEVEL=" + log,
					"PORT=5000",
					"PLUGIN=BasicPlugin",
				},
				Address: "192.168.1.1",
				Network: "pcp",
				OS:      rethink.PluginOSAll,
				Ports: []swarm.PortConfig{swarm.PortConfig{
					Protocol:      swarm.PortConfigProtocolTCP,
					TargetPort:    uint32(5000),
					PublishedPort: uint32(5000),
					PublishMode:   swarm.PortConfigPublishModeIngress,
				}},
				ServiceName: "BasicPluginService",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pluginToConfig(tt.args.plugin)
			if (err != nil) != tt.wantErr {
				t.Errorf("pluginToConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_selectChange(t *testing.T) {

	ctx := context.TODO()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	nodeInspect, err := dockerClient.NodeList(ctx, types.NodeListOptions{})
	nodeIP := nodeInspect[0].Status.Addr

	netRes, err := dockerClient.NetworkCreate(ctx, "pcp", types.NetworkCreate{
		Driver:     "overlay",
		Attachable: true,
	})
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	type args struct {
		plugin rethink.Plugin
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Basic plugin start (Harness)",
			args: args{
				plugin: rethink.Plugin{
					Name:          "Harness",
					ServiceID:     "",
					ServiceName:   "HarnessService1",
					DesiredState:  "Activate",
					State:         "Available",
					Address:       "",
					ExternalPorts: []string{"5000/tcp"},
					InternalPorts: []string{"5000/tcp"},
					OS:            rethink.PluginOSAll,
					Environment:   []string{},
				},
			},
			wantErr: false,
		},
		{
			name: "Advanced plugin start (Harness)",
			args: args{
				plugin: rethink.Plugin{
					Name:          "Harness",
					ServiceID:     "",
					ServiceName:   "HarnessService2",
					DesiredState:  "Activate",
					State:         "Available",
					Address:       "",
					ExternalPorts: []string{"5001/tcp"},
					InternalPorts: []string{"5001/tcp"},
					OS:            rethink.PluginOSPosix,
					Environment: []string{
						"TEST1=TEST1",
						"TEST2=TEST2",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Basic plugin update (Harness)",
			args: args{
				plugin: rethink.Plugin{
					Name:          "Harness",
					ServiceID:     "",
					ServiceName:   "HarnessService1",
					DesiredState:  "Restart",
					State:         "Active",
					Address:       "",
					ExternalPorts: []string{"5000/tcp", "6000/tcp"},
					InternalPorts: []string{"5000/tcp", "6000/tcp"},
					OS:            rethink.PluginOSAll,
					Environment:   []string{},
				},
			},
			wantErr: false,
		},
		{
			name: "Advanced plugin update (Harness)",
			args: args{
				plugin: rethink.Plugin{
					Name:          "Harness",
					ServiceID:     "",
					ServiceName:   "HarnessService2",
					DesiredState:  "Restart",
					State:         "Active",
					Address:       "",
					ExternalPorts: []string{"5001/tcp", "9999/tcp"},
					InternalPorts: []string{"5001/tcp", "9999/tcp"},
					OS:            rethink.PluginOSPosix,
					Environment: []string{
						"TEST1=TEST1",
						"TEST2=TEST2",
						"TEST3=TEST3",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Select IP start (Harness)",
			args: args{
				plugin: rethink.Plugin{
					Name:          "Harness",
					ServiceID:     "",
					ServiceName:   "HarnessService3",
					DesiredState:  "Activate",
					State:         "Available",
					Address:       nodeIP,
					ExternalPorts: []string{"5002/tcp"},
					InternalPorts: []string{"5002/tcp"},
					OS:            rethink.PluginOSAll,
					Environment: []string{
						"TEST1=TEST1",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.plugin.DesiredState == "Restart" {
				tt.args.plugin.ServiceID = test.GetServiceID(ctx, dockerClient, tt.args.plugin.ServiceName)
			}
			err := selectChange(tt.args.plugin)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectChange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			services, err := dockerClient.ServiceList(ctx, types.ServiceListOptions{})
			if err != nil {
				t.Errorf("couldn't get services")
				return
			}
			found := false
			for _, service := range services {
				if service.Spec.Annotations.Name == tt.args.plugin.ServiceName {
					found = true
					err = nil
					err = validateService(tt.args.plugin, service, netRes.ID)
					if err != nil {
						t.Errorf("%v", err)
						return
					}
				}
			}
			assert.True(t, found)
		})
	}

	//Docker cleanup
	if err := test.DockerCleanUp(ctx, dockerClient, netRes.ID); err != nil {
		t.Errorf("cleanup error: %v", err)
	}
}
