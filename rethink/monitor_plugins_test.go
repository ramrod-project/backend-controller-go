package rethink

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
)

func getBrainImage() string {
	var stringBuf bytes.Buffer

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = os.Getenv("TRAVIS_BRANCH")
	}
	if tag == "" {
		tag = "latest"
	}

	stringBuf.WriteString("ramrodpcp/database-brain:")
	stringBuf.WriteString(tag)

	return stringBuf.String()
}

var brainSpec = swarm.ServiceSpec{
	Annotations: swarm.Annotations{
		Name: "rethinkdb",
	},
	TaskTemplate: swarm.TaskSpec{
		ContainerSpec: swarm.ContainerSpec{
			DNSConfig: &swarm.DNSConfig{},
			Image:     getBrainImage(),
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

func Test_newPlugin(t *testing.T) {
	type args struct {
		change map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    *Plugin
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newPlugin(tt.args.change)
			if (err != nil) != tt.wantErr {
				t.Errorf("newPlugin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newPlugin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_watchChanges(t *testing.T) {

	ctx := context.TODO()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// Start brain
	result, err := dockerClient.ServiceCreate(ctx, brainSpec, types.ServiceCreateOptions{})
	if err != nil {
		t.Errorf("%v", err)
		return
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
				return
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

	testPlugin := map[string]interface{}{
		"Name":          "TestPlugin",
		"ServiceID":     "",
		"ServiceName":   "",
		"DesiredState":  string(DesiredStateNull),
		"State":         string(StateAvailable),
		"Interface":     "192.168.1.1",
		"ExternalPorts": []string{"1080/tcp"},
		"InternalPorts": []string{"1080/tcp"},
		"OS":            string(PluginOSAll),
	}
	_, err = r.DB("Controller").Table("Plugins").Insert(testPlugin).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// Test cases
	tests := []struct {
		name   string
		change map[string]interface{}
		want   Plugin
	}{
		{
			name: "Servicename change",
			change: map[string]interface{}{
				"ServiceName": "TestPluginService",
			},
			want: Plugin{
				Name:          "TestPlugin",
				ServiceID:     "",
				ServiceName:   "TestPluginService",
				DesiredState:  DesiredStateNull,
				State:         StateAvailable,
				Interface:     "192.168.1.1",
				ExternalPorts: []string{"1080/tcp"},
				InternalPorts: []string{"1080/tcp"},
				OS:            PluginOSAll,
			},
		},
		{
			name: "ServiceID add",
			change: map[string]interface{}{
				"ServiceID": "some-random-id",
			},
			want: Plugin{
				Name:          "TestPlugin",
				ServiceID:     "some-random-id",
				ServiceName:   "TestPluginService",
				DesiredState:  DesiredStateNull,
				State:         StateAvailable,
				Interface:     "192.168.1.1",
				ExternalPorts: []string{"1080/tcp"},
				InternalPorts: []string{"1080/tcp"},
				OS:            PluginOSAll,
			},
		},
		{
			name: "DesiredState change",
			change: map[string]interface{}{
				"DesiredState": "Activate",
			},
			want: Plugin{
				Name:          "TestPlugin",
				ServiceID:     "some-random-id",
				ServiceName:   "TestPluginService",
				DesiredState:  DesiredStateActivate,
				State:         StateAvailable,
				Interface:     "192.168.1.1",
				ExternalPorts: []string{"1080/tcp"},
				InternalPorts: []string{"1080/tcp"},
				OS:            PluginOSAll,
			},
		},
		{
			name: "State change",
			change: map[string]interface{}{
				"DesiredState": "",
				"State":        "Active",
			},
			want: Plugin{
				Name:          "TestPlugin",
				ServiceID:     "some-random-id",
				ServiceName:   "TestPluginService",
				DesiredState:  DesiredStateNull,
				State:         StateActive,
				Interface:     "192.168.1.1",
				ExternalPorts: []string{"1080/tcp"},
				InternalPorts: []string{"1080/tcp"},
				OS:            PluginOSAll,
			},
		},
		{
			name: "Port change",
			change: map[string]interface{}{
				"ExternalPorts": []string{"1080/tcp", "2000/tcp"},
				"InternalPorts": []string{"1080/tcp", "2000/tcp"},
			},
			want: Plugin{
				Name:          "TestPlugin",
				ServiceID:     "some-random-id",
				ServiceName:   "TestPluginService",
				DesiredState:  DesiredStateNull,
				State:         StateActive,
				Interface:     "192.168.1.1",
				ExternalPorts: []string{"1080/tcp", "2000/tcp"},
				InternalPorts: []string{"1080/tcp", "2000/tcp"},
				OS:            PluginOSAll,
			},
		},
		{
			name: "Deactivate",
			change: map[string]interface{}{
				"DesiredState": "Stop",
			},
			want: Plugin{
				Name:          "TestPlugin",
				ServiceID:     "some-random-id",
				ServiceName:   "TestPluginService",
				DesiredState:  DesiredStateStop,
				State:         StateActive,
				Interface:     "192.168.1.1",
				ExternalPorts: []string{"1080/tcp", "2000/tcp"},
				InternalPorts: []string{"1080/tcp", "2000/tcp"},
				OS:            PluginOSAll,
			},
		},
		{
			name: "Die",
			change: map[string]interface{}{
				"DesiredState": "",
				"State":        "Stopped",
			},
			want: Plugin{
				Name:          "TestPlugin",
				ServiceID:     "some-random-id",
				ServiceName:   "TestPluginService",
				DesiredState:  DesiredStateNull,
				State:         StateStopped,
				Interface:     "192.168.1.1",
				ExternalPorts: []string{"1080/tcp", "2000/tcp"},
				InternalPorts: []string{"1080/tcp", "2000/tcp"},
				OS:            PluginOSAll,
			},
		},
	}
	filter := map[string]string{"Name": "TestPlugin"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate cursor for changefeed
			c, err := r.DB("Controller").Table("Plugins").Changes().Run(session)
			if err != nil {
				t.Errorf("%v", err)
			}
			// Create channel and start goroutine
			resChan, errChan := watchChanges(c)
			// Insert change into database
			_, err = r.DB("Controller").Table("Plugins").Filter(filter).Update(tt.change).RunWrite(session)
			if err != nil {
				t.Errorf("%v", err)
			}
			select {
			case recvData := <-resChan:
				assert.Equal(t, recvData, tt.want)
			case recvErr := <-errChan:
				t.Errorf("%v", recvErr)
			default:
				t.Errorf("no messages received on either channel")
			}
			// Close cursor to stop goroutine
			c.Close()
		})
	}
	dockerClient.ServiceRemove(ctx, result.ID)
}

func TestMonitorPlugins(t *testing.T) {
	tests := []struct {
		name  string
		want  <-chan Plugin
		want1 <-chan error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := MonitorPlugins()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MonitorPlugins() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("MonitorPlugins() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_getRethinkHost(t *testing.T) {
	oldEnv := os.Getenv("STAGE")
	tests := []struct {
		name string
		env  string
		want string
	}{
		{
			name: "Dev",
			env:  "DEV",
			want: "rethinkdb",
		},
		{
			name: "Prod",
			env:  "PROD",
			want: "rethinkdb",
		},
		{
			name: "Testing",
			env:  "TESTING",
			want: "localhost",
		},
		{
			name: "Nil",
			env:  "",
			want: "rethinkdb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("STAGE", tt.env)
			if got := getRethinkHost(); got != tt.want {
				t.Errorf("getRethinkHost() = %v, want %v", got, tt.want)
			}
		})
	}
	os.Setenv("STAGE", oldEnv)
}
