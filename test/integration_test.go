package test

import (
	"context"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
)

func Test_Integration(t *testing.T) {

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

	var intBrainSpec = BrainSpec
	intBrainSpec.Networks = []swarm.NetworkAttachmentConfig{
		swarm.NetworkAttachmentConfig{
			Target:  "pcp",
			Aliases: []string{"rethinkdb"},
		},
	}

	// Start the brain
	session, brainID, err := StartBrain(ctx, t, dockerClient, intBrainSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// Start the controller
	contID, err := StartIntegrationTestService(ctx, dockerClient, controllerSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	var serviceIDs = []string{brainID, contID}

	time.Sleep(10 * time.Second)

	tests := []struct {
		name string
		run  func(t *testing.T) bool
	}{
		{
			name: "Startup ports",
			run: func(t *testing.T) bool {
				var ips []string
				var portEntries []string
				cursor, err := r.DB("Controller").Table("Ports").Run(session)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}

				// Get all port entry addresses from the db
				var doc map[string]interface{}
				for cursor.Next(&doc) {
					log.Printf("DB entry: %+v", doc)
					portEntries = append(portEntries, doc["Address"].(string))
				}

				// get local interfaces from node
				ifaces, err := net.Interfaces()
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				for _, i := range ifaces {
					addrs, err := i.Addrs()
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					for _, addr := range addrs {
						ips = append(ips, strings.Split(addr.String(), "/")[0])
					}
				}

				found := false
				for _, ip := range ips {
					for _, pEntry := range portEntries {
						if pEntry == ip {
							found = true
							break
						}
					}
					if found {
						return found
					}
				}

				if !found {
					t.Errorf("None of %v found in db.", ips)
				}
				return found
			},
		},
		{
			name: "Startup plugins",
			run: func(t *testing.T) bool {
				return true
			},
		},
		{
			name: "Update service",
			run: func(t *testing.T) bool {
				return true
			},
		},
		{
			name: "Create another service",
			run: func(t *testing.T) bool {
				return true
			},
		},
		{
			name: "Stop services",
			run: func(t *testing.T) bool {
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.run(t))
		})
	}

	// Service cleanup
	for _, service := range serviceIDs {
		KillService(ctx, dockerClient, service)
	}

	// Docker cleanup
	if err := DockerCleanUp(ctx, dockerClient, netRes.ID); err != nil {
		t.Errorf("cleanup error: %v", err)
	}
}
