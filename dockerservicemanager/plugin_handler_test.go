package dockerservicemanager

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	rethink "github.com/ramrod-project/backend-controller-go/rethink"
	"github.com/stretchr/testify/assert"
)

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
			name: "Basic plugin (Harness)",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				}
			}
			assert.True(t, found)
		})
	}

	//Docker cleanup
	services, err := dockerClient.ServiceList(ctx, types.ServiceListOptions{})
	for _, v := range services {
		if v.ID == "" {
			continue
		}
		err := dockerClient.ServiceRemove(ctx, v.ID)
		if err != nil {
			t.Errorf("%v", err)
		}
	}
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
	for _, c := range containers {
		if c.ID == "" {
			continue
		}
		err := dockerClient.ContainerKill(ctx, c.ID, "SIGKILL")
		if err != nil {
			t.Errorf("%v", err)
		}
		err = dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			t.Errorf("%v", err)
		}
	}
	start := time.Now()
	for time.Since(start) < 5*time.Second {
		err := dockerClient.NetworkRemove(ctx, netRes.ID)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
}
