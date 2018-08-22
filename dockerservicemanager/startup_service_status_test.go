package dockerservicemanager

import (
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/ramrod-project/backend-controller-go/rethink"
	"github.com/stretchr/testify/assert"
)

func Test_serviceToEntry(t *testing.T) {

	tests := []struct {
		name    string
		dbEntry map[string]interface{}
		svc     swarm.Service
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "posix",
			dbEntry: map[string]interface{}{},
			svc: swarm.Service{
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name: "TestService",
					},
					TaskTemplate: swarm.TaskSpec{
						ContainerSpec: swarm.ContainerSpec{
							Env: []string{"PLUGIN=TestPlugin"},
						},
						Placement: &swarm.Placement{
							Constraints: []string{"node.labels.os==posix"},
						},
					},
				},
				ID: "testid",
				Endpoint: swarm.Endpoint{
					Ports: []swarm.PortConfig{
						swarm.PortConfig{
							Protocol:      swarm.PortConfigProtocolTCP,
							PublishedPort: 5000,
							TargetPort:    5000,
						},
					},
				},
			},
			want: map[string]interface{}{
				"Name":          "TestPlugin",
				"ServiceID":     "testid",
				"ServiceName":   "TestService",
				"DesiredState":  "",
				"State":         "Active",
				"Interface":     "",
				"ExternalPorts": []string{"5000/tcp"},
				"InternalPorts": []string{"5000/tcp"},
				"OS":            string(rethink.PluginOSPosix),
				"Environment":   []string{},
			},
		},
		{
			name:    "nt",
			dbEntry: map[string]interface{}{},
			svc: swarm.Service{
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name: "TestServiceWin",
					},
					TaskTemplate: swarm.TaskSpec{
						ContainerSpec: swarm.ContainerSpec{
							Env: []string{"PLUGIN=TestPluginWin"},
						},
						Placement: &swarm.Placement{
							Constraints: []string{"node.labels.os==nt"},
						},
					},
				},
				ID: "testidwin",
				Endpoint: swarm.Endpoint{
					Ports: []swarm.PortConfig{
						swarm.PortConfig{
							Protocol:      swarm.PortConfigProtocolUDP,
							PublishedPort: 7000,
							TargetPort:    7000,
						},
					},
				},
			},
			want: map[string]interface{}{
				"Name":          "TestPluginWin",
				"ServiceID":     "testidwin",
				"ServiceName":   "TestServiceWin",
				"DesiredState":  "",
				"State":         "Active",
				"Interface":     "",
				"ExternalPorts": []string{"7000/udp"},
				"InternalPorts": []string{"7000/udp"},
				"OS":            string(rethink.PluginOSWindows),
				"Environment":   []string{},
			},
		},
		{
			name:    "posix adv",
			dbEntry: map[string]interface{}{},
			svc: swarm.Service{
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name: "TestServiceAdv",
					},
					TaskTemplate: swarm.TaskSpec{
						ContainerSpec: swarm.ContainerSpec{
							Env: []string{"PLUGIN=TestPluginAdv", "TESTENV=TESTADV"},
						},
						Placement: &swarm.Placement{
							Constraints: []string{"node.labels.os==posix"},
						},
					},
				},
				ID: "testidadv",
				Endpoint: swarm.Endpoint{
					Ports: []swarm.PortConfig{
						swarm.PortConfig{
							Protocol:      swarm.PortConfigProtocolTCP,
							PublishedPort: 9999,
							TargetPort:    9999,
						},
						swarm.PortConfig{
							Protocol:      swarm.PortConfigProtocolUDP,
							PublishedPort: 5555,
							TargetPort:    5555,
						},
					},
				},
			},
			want: map[string]interface{}{
				"Name":          "TestPluginAdv",
				"ServiceID":     "testidadv",
				"ServiceName":   "TestServiceAdv",
				"DesiredState":  "",
				"State":         "Active",
				"Interface":     "",
				"ExternalPorts": []string{"9999/tcp", "5555/udp"},
				"InternalPorts": []string{"9999/tcp", "5555/udp"},
				"OS":            string(rethink.PluginOSPosix),
				"Environment":   []string{"TESTENV=TESTADV"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := serviceToEntry(tt.svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("serviceToEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, got, tt.want)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("serviceToEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStartupServiceStatus(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := StartupServiceStatus(); (err != nil) != tt.wantErr {
				t.Errorf("StartupServiceStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
