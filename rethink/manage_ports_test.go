package rethink

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/test"
	r "gopkg.in/gorethink/gorethink.v4"
)

func TestAddPort(t *testing.T) {
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

	var intBrainSpec = test.BrainSpec
	intBrainSpec.Networks = []swarm.NetworkAttachmentConfig{
		swarm.NetworkAttachmentConfig{
			Target:  "pcp",
			Aliases: []string{"rethinkdb"},
		},
	}

	session, brainID, err := test.StartBrain(ctx, t, dockerClient, test.BrainSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	time.Sleep(10 * time.Second)

	testPort := map[string]interface{}{
		"Interface":    "192.168.1.1",
		"TCPPorts":     []string{"6003", "6002"},
		"UDPPorts":     []string{},
		"NodeHostName": "Docker",
		"OS":           "posix",
	}
	_, err = r.DB("Controller").Table("Ports").Insert(testPort).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	type args struct {
		IPaddr   string
		newPort  string
		protocol string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Test adding a port",
			args: args{
				IPaddr:   "192.168.1.1",
				newPort:  "9990",
				protocol: "tcp",
			},
			wantErr: false,
		}, {
			name: "Test adding a port to empty",
			args: args{
				IPaddr:   "192.168.1.1",
				newPort:  "5178",
				protocol: "udp",
			},
			wantErr: false,
		}, {
			name: "Test adding a duplicate port",
			args: args{
				IPaddr:   "192.168.1.1",
				newPort:  "9990",
				protocol: "tcp",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := AddPort(tt.args.IPaddr, tt.args.newPort, tt.args.protocol); (err != nil) != tt.wantErr {
				t.Errorf("AddPort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	testRes := make(map[string]interface{})
	testRes = getCurrentEntry("192.168.1.1", session)

	var (
		newTCP []string
		newUDP []string
	)

	for _, tcpPort := range testRes["TCPPorts"].([]interface{}) {
		newTCP = append(newTCP, tcpPort.(string))
	}
	for _, udpPort := range testRes["UDPPorts"].([]interface{}) {
		newUDP = append(newUDP, udpPort.(string))
	}

	if !contains(newTCP, "9990") {
		t.Errorf("9990 was not added")
	}
	if !contains(newUDP, "5178") {
		t.Errorf("failed to add to empty string")
	}
	test.KillService(ctx, dockerClient, brainID)
	// Docker cleanup
	if err := test.DockerCleanUp(ctx, dockerClient, netRes.ID); err != nil {
		t.Errorf("cleanup error: %v", err)
	}
}

func TestRemovePort(t *testing.T) {
	ctx := context.TODO()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	session, brainID, err := test.StartBrain(ctx, t, dockerClient, test.BrainSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	testPort := map[string]interface{}{
		"Interface":    "192.168.1.1",
		"TCPPorts":     []string{"6003", "6002"},
		"UDPPorts":     []string{"8000"},
		"NodeHostName": "Docker",
		"OS":           "posix",
	}
	_, err = r.DB("Controller").Table("Ports").Insert(testPort).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	type args struct {
		IPaddr   string
		remPort  string
		protocol string
	}
	tests := []struct {
		name    string
		args    args
		err     error
		wantErr bool
	}{
		{
			name: "delete a port",
			args: args{
				IPaddr:   "192.168.1.1",
				remPort:  "6003",
				protocol: "tcp",
			},
			wantErr: false,
		},
		{
			name: "delete a port that does not exist",
			args: args{
				IPaddr:   "192.168.1.1",
				remPort:  "6003",
				protocol: "tcp",
			},
			err:     errors.New("port doesn't exits"),
			wantErr: true,
		},
		{
			name: "delete the last port",
			args: args{
				IPaddr:   "192.168.1.1",
				remPort:  "8000",
				protocol: "udp",
			},
			wantErr: false,
		},
		{
			name: "delete a port in empty list",
			args: args{
				IPaddr:   "192.168.1.1",
				remPort:  "9000",
				protocol: "udp",
			},
			err:     errors.New("port doesn't exits"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RemovePort(tt.args.IPaddr, tt.args.remPort, tt.args.protocol); (err != nil) != tt.wantErr {
				t.Errorf("RemovePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	testRes := make(map[string]interface{})
	testRes = getCurrentEntry("192.168.1.1", session)

	var (
		newTCP []string
	)

	for _, tcpPort := range testRes["TCPPorts"].([]interface{}) {
		newTCP = append(newTCP, tcpPort.(string))
	}

	if contains(newTCP, "6003") {
		t.Errorf("port 6003 was not deleted")
	}
	test.KillService(ctx, dockerClient, brainID)
}
