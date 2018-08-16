package rethink

import (
	"context"
	"testing"

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

	session, brainID, err := test.StartBrain(ctx, t, dockerClient, test.BrainSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

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
	if !contains(testRes["TCPPorts"].([]string), "9990") {
		t.Errorf("9990 was not added")
	}
	if !contains(testRes["UPDPorts"].([]string), "5178") {
		t.Errorf("failed to add to empty string")
	}
	test.KillService(ctx, dockerClient, brainID)
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
		}, {
			name: "delete a port that does not exist",
			args: args{
				IPaddr:   "192.168.1.1",
				remPort:  "6003",
				protocol: "tcp",
			},
			wantErr: true,
		}, {
			name: "delete the last port",
			args: args{
				IPaddr:   "192.168.1.1",
				remPort:  "8000",
				protocol: "udp",
			},
			wantErr: false,
		}, {
			name: "delete a port in empty list",
			args: args{
				IPaddr:   "192.168.1.1",
				remPort:  "9000",
				protocol: "udp",
			},
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
	if contains(testRes["TCPPorts"].([]string), "6003") {
		t.Errorf("port 6003 was not deleted")
	}
	test.KillService(ctx, dockerClient, brainID)
}
