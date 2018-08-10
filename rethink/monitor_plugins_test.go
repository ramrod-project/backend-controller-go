package rethink

import (
	"bytes"
	"context"
	"fmt"
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

func startBrain(ctx context.Context, t *testing.T, dockerClient *client.Client) (*r.Session, string, error) {
	// Start brain
	result, err := dockerClient.ServiceCreate(ctx, brainSpec, types.ServiceCreateOptions{})
	if err != nil {
		t.Errorf("%v", err)
		return nil, "", err
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
				return nil, "", err
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
	return session, result.ID, err
}

func killBrain(ctx context.Context, dockerClient *client.Client, brainID string) {
	start := time.Now()
	for time.Since(start) < 10*time.Second {
		err := dockerClient.ServiceRemove(ctx, brainID)
		if err != nil {
			break
		}
		time.Sleep(time.Second)
	}
	for time.Since(start) < 15*time.Second {
		containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{})
		if err == nil {
			if len(containers) == 0 {
				break
			}
			for _, c := range containers {
				err = dockerClient.ContainerKill(ctx, c.ID, "")
				if err == nil {
					dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})
				}
			}
		}
		time.Sleep(time.Second)
	}
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
		err     error
	}{
		{
			name: "Basic good plugin",
			args: args{
				change: map[string]interface{}{
					"Name":          "TestPlugin",
					"ServiceID":     "",
					"ServiceName":   "TestPlugin-5000",
					"DesiredState":  "",
					"State":         "Available",
					"Interface":     "192.168.1.1",
					"ExternalPorts": []interface{}{"5000/tcp"},
					"InternalPorts": []interface{}{"5000/tcp"},
					"OS":            "all",
					"Environment":   []string{},
				},
			},
			want: &Plugin{
				Name:          "TestPlugin",
				ServiceID:     "",
				ServiceName:   "TestPlugin-5000",
				DesiredState:  DesiredStateNull,
				State:         StateAvailable,
				Address:       "192.168.1.1",
				ExternalPorts: []string{"5000/tcp"},
				InternalPorts: []string{"5000/tcp"},
				OS:            PluginOSAll,
				Environment:   []string{},
			},
			wantErr: false,
		},
		{
			name: "Bad no ServiceName",
			args: args{
				change: map[string]interface{}{
					"Name":          "TestPlugin",
					"ServiceID":     "",
					"ServiceName":   "",
					"DesiredState":  "",
					"State":         "Available",
					"Interface":     "192.168.1.1",
					"ExternalPorts": []interface{}{"5000/tcp"},
					"InternalPorts": []interface{}{"5000/tcp"},
					"OS":            "all",
					"Environment":   []string{},
				},
			},
			want:    &Plugin{},
			wantErr: true,
			err:     NewControllerError("plugin service must have ServiceName"),
		},
		{
			name: "Bad desired state",
			args: args{
				change: map[string]interface{}{
					"Name":          "TestPlugin",
					"ServiceID":     "",
					"ServiceName":   "TestPlugin-5000",
					"DesiredState":  "blah",
					"State":         "Available",
					"Interface":     "192.168.1.1",
					"ExternalPorts": []interface{}{"5000/tcp"},
					"InternalPorts": []interface{}{"5000/tcp"},
					"OS":            "all",
					"Environment":   []string{},
				},
			},
			want:    &Plugin{},
			wantErr: true,
			err:     NewControllerError(fmt.Sprintf("invalid desired state %v sent", "blah")),
		},
		{
			name: "Bad state",
			args: args{
				change: map[string]interface{}{
					"Name":          "TestPlugin",
					"ServiceID":     "",
					"ServiceName":   "TestPlugin-5000",
					"DesiredState":  "",
					"State":         "",
					"Interface":     "192.168.1.1",
					"ExternalPorts": []interface{}{"5000/tcp"},
					"InternalPorts": []interface{}{"5000/tcp"},
					"OS":            "all",
					"Environment":   []string{},
				},
			},
			want:    &Plugin{},
			wantErr: true,
			err:     NewControllerError(fmt.Sprintf("invalid state %v sent", "")),
		},
		{
			name: "Plugin with more stuff",
			args: args{
				change: map[string]interface{}{
					"Name":          "TestPluginAdvanced",
					"ServiceID":     "b972q4567qe8rhgq3503q6",
					"ServiceName":   "TestPlugin-5005",
					"DesiredState":  "Restart",
					"State":         "Active",
					"Interface":     "192.168.1.1",
					"ExternalPorts": []interface{}{"5005/tcp", "5006/tcp", "5007/tcp", "5008/tcp"},
					"InternalPorts": []interface{}{"5005/tcp", "5006/tcp", "5007/tcp", "5008/tcp"},
					"OS":            "nt",
					"Environment":   []string{"TEST=TEST", "TEST2=TEST2"},
				},
			},
			want: &Plugin{
				Name:          "TestPluginAdvanced",
				ServiceID:     "b972q4567qe8rhgq3503q6",
				ServiceName:   "TestPlugin-5005",
				DesiredState:  DesiredStateRestart,
				State:         StateActive,
				Address:       "192.168.1.1",
				ExternalPorts: []string{"5005/tcp", "5006/tcp", "5007/tcp", "5008/tcp"},
				InternalPorts: []string{"5005/tcp", "5006/tcp", "5007/tcp", "5008/tcp"},
				OS:            PluginOSWindows,
				Environment:   []string{"TEST=TEST", "TEST2=TEST2"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newPlugin(tt.args.change)
			if (err != nil) != tt.wantErr {
				t.Errorf("newPlugin() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				assert.Equal(t, tt.err, err)
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

	session, brainID, err := startBrain(ctx, t, dockerClient)
	if err != nil {
		t.Errorf("%v", err)
		return
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
		"Environment":   []string{"TEST=TEST", "TEST2=TEST2"},
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
				Address:       "192.168.1.1",
				ExternalPorts: []string{"1080/tcp"},
				InternalPorts: []string{"1080/tcp"},
				OS:            PluginOSAll,
				Environment:   []string{"TEST=TEST", "TEST2=TEST2"},
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
				Address:       "192.168.1.1",
				ExternalPorts: []string{"1080/tcp"},
				InternalPorts: []string{"1080/tcp"},
				OS:            PluginOSAll,
				Environment:   []string{"TEST=TEST", "TEST2=TEST2"},
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
				Address:       "192.168.1.1",
				ExternalPorts: []string{"1080/tcp"},
				InternalPorts: []string{"1080/tcp"},
				OS:            PluginOSAll,
				Environment:   []string{"TEST=TEST", "TEST2=TEST2"},
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
				Address:       "192.168.1.1",
				ExternalPorts: []string{"1080/tcp"},
				InternalPorts: []string{"1080/tcp"},
				OS:            PluginOSAll,
				Environment:   []string{"TEST=TEST", "TEST2=TEST2"},
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
				Address:       "192.168.1.1",
				ExternalPorts: []string{"1080/tcp", "2000/tcp"},
				InternalPorts: []string{"1080/tcp", "2000/tcp"},
				OS:            PluginOSAll,
				Environment:   []string{"TEST=TEST", "TEST2=TEST2"},
			},
		},
		{
			name: "Env change",
			change: map[string]interface{}{
				"Environment": []string{"TEST1=TEST1", "TEST3=TEST3", "TEST4=TEST4"},
			},
			want: Plugin{
				Name:          "TestPlugin",
				ServiceID:     "some-random-id",
				ServiceName:   "TestPluginService",
				DesiredState:  DesiredStateNull,
				State:         StateActive,
				Address:       "192.168.1.1",
				ExternalPorts: []string{"1080/tcp", "2000/tcp"},
				InternalPorts: []string{"1080/tcp", "2000/tcp"},
				OS:            PluginOSAll,
				Environment:   []string{"TEST1=TEST1", "TEST3=TEST3", "TEST4=TEST4"},
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
				Address:       "192.168.1.1",
				ExternalPorts: []string{"1080/tcp", "2000/tcp"},
				InternalPorts: []string{"1080/tcp", "2000/tcp"},
				OS:            PluginOSAll,
				Environment:   []string{"TEST1=TEST1", "TEST3=TEST3", "TEST4=TEST4"},
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
				Address:       "192.168.1.1",
				ExternalPorts: []string{"1080/tcp", "2000/tcp"},
				InternalPorts: []string{"1080/tcp", "2000/tcp"},
				OS:            PluginOSAll,
				Environment:   []string{"TEST1=TEST1", "TEST3=TEST3", "TEST4=TEST4"},
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
				assert.Equal(t, tt.want, recvData)
			case recvErr := <-errChan:
				t.Errorf("%v", recvErr)
			default:
				t.Errorf("no messages received on either channel")
			}
			// Close cursor to stop goroutine
			c.Close()
		})
	}
	killBrain(ctx, dockerClient, brainID)
}

func TestMonitorPlugins(t *testing.T) {
	oldEnv := os.Getenv("STAGE")
	os.Setenv("STAGE", "TESTING")

	ctx := context.TODO()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	session, brainID, err := startBrain(ctx, t, dockerClient)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	tests := []struct {
		name    string
		plugin  map[string]interface{}
		change  map[string]interface{}
		filter  map[string]string
		want    Plugin
		wantErr bool
		err     error
	}{
		{
			name: "Add first plugin",
			plugin: map[string]interface{}{
				"Name":          "TestPlugin1",
				"ServiceID":     "",
				"ServiceName":   "TestPlugin1Service",
				"DesiredState":  "",
				"State":         "Available",
				"Interface":     "192.168.1.1",
				"ExternalPorts": []interface{}{"999/tcp"},
				"InternalPorts": []interface{}{"999/tcp"},
				"OS":            "nt",
			},
			want: Plugin{
				Name:          "TestPlugin1",
				ServiceID:     "",
				ServiceName:   "TestPlugin1Service",
				DesiredState:  DesiredStateNull,
				State:         StateAvailable,
				Address:       "192.168.1.1",
				ExternalPorts: []string{"999/tcp"},
				InternalPorts: []string{"999/tcp"},
				OS:            PluginOSWindows,
				Environment:   []string{},
			},
			wantErr: false,
		},
		{
			name: "Add second plugin",
			plugin: map[string]interface{}{
				"Name":          "TestPlugin2",
				"ServiceID":     "",
				"ServiceName":   "TestPlugin2Service",
				"DesiredState":  "",
				"State":         "Available",
				"Interface":     "192.168.1.2",
				"ExternalPorts": []interface{}{"444/tcp", "444/udp"},
				"InternalPorts": []interface{}{"444/tcp", "444/udp"},
				"OS":            "all",
				"Environment":   []string{"TEST=TEST"},
			},
			want: Plugin{
				Name:          "TestPlugin2",
				ServiceID:     "",
				ServiceName:   "TestPlugin2Service",
				DesiredState:  DesiredStateNull,
				State:         StateAvailable,
				Address:       "192.168.1.2",
				ExternalPorts: []string{"444/tcp", "444/udp"},
				InternalPorts: []string{"444/tcp", "444/udp"},
				OS:            PluginOSAll,
				Environment:   []string{"TEST=TEST"},
			},
			wantErr: false,
		},
		{
			name: "Change first plugin",
			change: map[string]interface{}{
				"DesiredState": "Activate",
			},
			filter: map[string]string{
				"ServiceName": "TestPlugin1Service",
			},
			want: Plugin{
				Name:          "TestPlugin1",
				ServiceID:     "",
				ServiceName:   "TestPlugin1Service",
				DesiredState:  DesiredStateActivate,
				State:         StateAvailable,
				Address:       "192.168.1.1",
				ExternalPorts: []string{"999/tcp"},
				InternalPorts: []string{"999/tcp"},
				OS:            PluginOSWindows,
				Environment:   []string{},
			},
			wantErr: false,
		},
		{
			name: "Change second plugin",
			change: map[string]interface{}{
				"ServiceID": "q3hyt80qeh5ygt8hbeq8itjhgq9854t",
			},
			filter: map[string]string{
				"ServiceName": "TestPlugin2Service",
			},
			want: Plugin{
				Name:          "TestPlugin2",
				ServiceID:     "q3hyt80qeh5ygt8hbeq8itjhgq9854t",
				ServiceName:   "TestPlugin2Service",
				DesiredState:  DesiredStateNull,
				State:         StateAvailable,
				Address:       "192.168.1.2",
				ExternalPorts: []string{"444/tcp", "444/udp"},
				InternalPorts: []string{"444/tcp", "444/udp"},
				OS:            PluginOSAll,
				Environment:   []string{"TEST=TEST"},
			},
			wantErr: false,
		},
		{
			name: "Bad change first plugin",
			change: map[string]interface{}{
				"Name": "",
			},
			filter: map[string]string{
				"ServiceName": "TestPlugin1Service",
			},
			want:    Plugin{},
			wantErr: true,
			err:     NewControllerError("plugin name must not be blank"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create channel and start goroutine
			dbChan, errChan := MonitorPlugins()
			var err error
			// Insert new plugin
			if tt.plugin != nil {
				_, err = r.DB("Controller").Table("Plugins").Insert(tt.plugin).RunWrite(session)
			} else {
				// Or insert change
				_, err = r.DB("Controller").Table("Plugins").Filter(tt.filter).Update(tt.change).RunWrite(session)
			}
			if err != nil {
				t.Errorf("%v", err)
			}
			if tt.wantErr {
				select {
				case recvErr := <-errChan:
					assert.Equal(t, tt.err, recvErr)
				default:
					t.Errorf("no error message received")
				}
			} else {
				select {
				case recvData := <-dbChan:
					assert.Equal(t, tt.want, recvData)
				case recvErr := <-errChan:
					assert.Equal(t, tt.err, recvErr)
				default:
					t.Errorf("no message received on either channel")
				}
			}
		})
	}
	killBrain(ctx, dockerClient, brainID)
	os.Setenv("STAGE", oldEnv)
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
			want: "127.0.0.1",
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
