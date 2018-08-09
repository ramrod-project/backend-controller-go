package dockerservicemanager

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
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

func Test_getNodes(t *testing.T) {

	ctx := context.TODO()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "Test getting the node and assigning",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getNodes()
			if (err != nil) != tt.wantErr {
				t.Errorf("getNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Len(t, got, 1)
			assert.IsType(t, "string", got[0]["Address"])
			assert.NotEqual(t, "", got[0]["Address"])
			assert.IsType(t, "string", got[0]["OS"])
			assert.NotEqual(t, "", got[0]["OS"])
			assert.IsType(t, "string", got[0]["NodeHostName"])
			assert.NotEqual(t, "", got[0]["NodeHostName"])
			assert.IsType(t, []string{}, got[0]["TCPPorts"])
			assert.Equal(t, []string{}, got[0]["TCPPorts"])
			assert.IsType(t, []string{}, got[0]["UDPPorts"])
			assert.Equal(t, []string{}, got[0]["UDPPorts"])
			res, err := dockerClient.NodeList(ctx, types.NodeListOptions{})
			if err != nil {
				t.Errorf("%v", err)
				return
			}
			assert.Equal(t, "posix", res[0].Spec.Annotations.Labels["os"])
			assert.Equal(t, got[0]["Address"], res[0].Spec.Annotations.Labels["ip"])
		})
	}
}

func Test_advertiseIPs(t *testing.T) {
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

	type args struct {
		entries []map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		err     error
	}{
		{
			name: "One node",
			args: args{
				entries: []map[string]interface{}{
					map[string]interface{}{
						"Address":      "192.168.1.1",
						"NodeHostName": "ubuntu",
						"OS":           "posix",
						"TCPPorts":     []string{},
						"UDPPorts":     []string{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Two node",
			args: args{
				entries: []map[string]interface{}{
					map[string]interface{}{
						"Address":      "192.168.1.1",
						"NodeHostName": "ubuntu",
						"OS":           "posix",
						"TCPPorts":     []string{},
						"UDPPorts":     []string{},
					},
					map[string]interface{}{
						"Address":      "192.168.1.2",
						"NodeHostName": "WIN1935U21",
						"OS":           "nt",
						"TCPPorts":     []string{},
						"UDPPorts":     []string{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "No node",
			args: args{
				entries: []map[string]interface{}{},
			},
			wantErr: true,
			err:     errors.New("no nodes to advertise"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := advertiseIPs(tt.args.entries); (err != nil) != tt.wantErr {
				t.Errorf("advertiseIPs() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				assert.Equal(t, tt.err, err)
				return
			}
			cursor, err := r.DB("Controller").Table("Ports").Run(session)
			if err != nil {
				t.Errorf("Rethink error: %v", err)
				return
			}
			var doc map[string]interface{}
			i := 0
			for cursor.Next(&doc) {
				index := 0
				if doc["OS"] == "nt" {
					index = 1
				}
				assert.Equal(t, tt.args.entries[index]["Address"], doc["Address"])
				assert.Equal(t, tt.args.entries[index]["NodeHostName"], doc["NodeHostName"])
				assert.Equal(t, tt.args.entries[index]["OS"], doc["OS"])
				assert.Equal(t, len(tt.args.entries[index]["TCPPorts"].([]string)), len(doc["TCPPorts"].([]interface{})))
				assert.Equal(t, len(tt.args.entries[index]["UDPPorts"].([]string)), len(doc["UDPPorts"].([]interface{})))
				for j, v := range doc["TCPPorts"].([]interface{}) {
					assert.Equal(t, tt.args.entries[index]["TCPPorts"].([]string)[j], v.(string))
				}
				for j, v := range doc["UDPPorts"].([]interface{}) {
					assert.Equal(t, tt.args.entries[index]["UDPPorts"].([]string)[j], v.(string))
				}
				i++
			}
			if i == 0 {
				t.Error("empty cursor result")
				return
			}
			_, err = r.DB("Controller").Table("Ports").Delete().Run(session)
			if err != nil {
				t.Errorf("%v", err)
				return
			}
		})
	}
	killBrain(ctx, dockerClient, brainID)
	os.Setenv("STAGE", oldEnv)
}
