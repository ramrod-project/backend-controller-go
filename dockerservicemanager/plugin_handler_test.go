package dockerservicemanager

import (
	"os"
	"testing"

	swarm "github.com/docker/docker/api/types/swarm"
	rethink "github.com/ramrod-project/backend-controller-go/rethink"
	"github.com/stretchr/testify/assert"
)

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

func Test_getEnvByKey(t *testing.T) {
	type args struct {
		k string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getEnvByKey(tt.args.k); got != tt.want {
				t.Errorf("getEnvByKey() = %v, want %v", got, tt.want)
			}
		})
	}
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := envString(tt.args.k, tt.args.v); got != tt.want {
				t.Errorf("envString() = %v, want %v", got, tt.want)
			}
		})
	}
}
func Test_selectChange(t *testing.T) {
	type args struct {
		plugin rethink.Plugin
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := selectChange(tt.args.plugin)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectChange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
