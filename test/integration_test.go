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
	"github.com/ramrod-project/backend-controller-go/rethink"
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

	time.Sleep(5 * time.Second)

	tests := []struct {
		name string
		run  func(t *testing.T) bool
		// Used if need to wait for a result to propogate
		wait func(t *testing.T, timeout time.Duration) bool
		// Set timeout for wait
		timeout time.Duration
	}{
		{
			name: "Startup ports",
			run: func(t *testing.T) bool {
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					ips []string
				)

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

				start := time.Now()

				for time.Now().Before(start.Add(timeout)) {
					// Get all port entry addresses from the db
					var portEntries []string
					var doc map[string]interface{}

					cursor, err := r.DB("Controller").Table("Ports").Run(session)
					if err != nil {
						t.Errorf("%v", err)
						return false
					}

					for cursor.Next(&doc) {
						log.Printf("Port DB entry: %+v", doc)
						portEntries = append(portEntries, doc["Address"].(string))
					}

					for _, ip := range ips {
						for _, pEntry := range portEntries {
							if pEntry == ip {
								return true
							}
						}
					}
					time.Sleep(time.Second)
				}
				t.Errorf("None of %v found in db.", ips)
				return false
			},
			timeout: 10 * time.Second,
		},
		{
			name: "Startup plugins",
			run: func(t *testing.T) bool {
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				start := time.Now()

				for time.Now().Before(start.Add(timeout)) {
					cursor, err := r.DB("Controller").Table("Plugins").Run(session)
					if err != nil {
						t.Errorf("%v", err)
						return false
					}

					// Get all plugins from the db (should only be one)
					var pluginEntries []map[string]interface{}
					var doc map[string]interface{}

					for cursor.Next(&doc) {
						log.Printf("Plugin DB entry: %+v", doc)
						pluginEntries = append(pluginEntries, doc)
					}

					if len(pluginEntries) < 1 {
						time.Sleep(time.Second)
						continue
					}

					if len(pluginEntries) != 1 {
						t.Errorf("shouldn't be %v plugins", len(pluginEntries))
						return false
					}

					assert.Equal(t, "Harness", pluginEntries[0]["Name"].(string))
					assert.Equal(t, "", pluginEntries[0]["ServiceID"].(string))
					assert.Equal(t, "", pluginEntries[0]["ServiceName"].(string))
					assert.Equal(t, string(rethink.DesiredStateNull), pluginEntries[0]["DesiredState"].(string))
					assert.Equal(t, string(rethink.StateAvailable), pluginEntries[0]["State"].(string))
					assert.Equal(t, "", pluginEntries[0]["Address"].(string))
					assert.Equal(t, string(rethink.PluginOSAll), pluginEntries[0]["OS"].(string))
					break
				}

				return true
			},
			timeout: 10 * time.Second,
		},
		{
			name: "Create service",
			run: func(t *testing.T) bool {
				_, err := r.DB("Controller").Table("Plugins").Insert(map[string]interface{}{
					"Name":          "Harness",
					"ServiceID":     "",
					"ServiceName":   "TestPlugin",
					"DesiredState":  "Activate",
					"State":         "Available",
					"Address":       "",
					"ExternalPorts": []string{"5000/tcp"},
					"InternalPorts": []string{"5000/tcp"},
					"OS":            string(rethink.PluginOSAll),
					"Environment":   []string{},
				}).Run(session)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}

				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					foundService, dbUpdated, serviceVerify bool = false, false, false
					doc                                    map[string]interface{}
					targetService                          swarm.Service
				)
				start := time.Now()

				for time.Now().Before(start.Add(timeout)) {
					services, _ := dockerClient.ServiceList(ctx, types.ServiceListOptions{})
					for _, service := range services {
						if service.Spec.Annotations.Name == "TestPlugin" {
							targetService = service
							log.Printf("Found service by ID %v", service.ID)
							foundService = true
							break
						}
					}
					if foundService {
						break
					}
					time.Sleep(time.Second)
				}

				var inspect swarm.Service
				for time.Now().Before(start.Add(timeout)) {
					time.Sleep(time.Second)
					// Check docker service
					inspect, _, _ = dockerClient.ServiceInspectWithRaw(ctx, targetService.ID)
					if *inspect.Spec.Mode.Replicated.Replicas != uint64(1) {
						continue
					}
					if len(inspect.Endpoint.Ports) < 1 {
						continue
					}
					if inspect.Endpoint.Ports[0].Protocol != swarm.PortConfigProtocolTCP {
						continue
					}
					if inspect.Endpoint.Ports[0].PublishedPort != uint32(5000) {
						continue
					}
					if inspect.Endpoint.Ports[0].TargetPort != uint32(5000) {
						continue
					}
					if inspect.Endpoint.Ports[0].PublishMode != swarm.PortConfigPublishModeIngress {
						continue
					}
					if len(inspect.Spec.TaskTemplate.Networks) < 1 {
						continue
					}
					if inspect.Spec.TaskTemplate.Networks[0].Target != netRes.ID {
						continue
					}
					log.Printf("Verified service %+v", inspect)
					serviceVerify = true
					break
				}

				for time.Now().Before(start.Add(timeout)) {
					// Check database entry
					cursor, err := r.DB("Controller").Table("Plugins").Run(session)
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					// Check cursor for the desired plugin entry
					for cursor.Next(&doc) {
						if doc["ServiceName"].(string) != "TestPlugin" {
							continue
						}
						if doc["ServiceID"].(string) == "" {
							log.Printf("empty id")
							break
						}
						if doc["State"].(string) != "Active" {
							log.Printf("bad state: %v", doc["State"])
							break
						}
						if doc["DesiredState"].(string) != "" {
							log.Printf("bad desiredstate: %v", doc["DesiredState"])
							break
						}
						log.Printf("DB update verified %+v", doc)
						dbUpdated = true
						break
					}
					if dbUpdated {
						break
					}
					time.Sleep(time.Second)
				}

				if !foundService || !serviceVerify {
					log.Printf("Inspect results: %+v", inspect)
					t.Errorf("Didn't find service before timeout")
				}
				if !dbUpdated {
					cursor, err := r.DB("Controller").Table("Plugins").Run(session)
					if err != nil {
						t.Errorf("%v", err)
					} else {
						var res map[string]interface{}
						for cursor.Next(&res) {
							log.Printf("DB item: %+v", res)
						}
					}
					t.Errorf("Didn't update database before timeout")
				}

				return foundService && dbUpdated && serviceVerify
			},
			timeout: 20 * time.Second,
		},
		{
			name: "Update service",
			run: func(t *testing.T) bool {
				_, err := r.DB("Controller").Table("Plugins").Filter(map[string]string{"ServiceName": "TestPlugin"}).Update(map[string]interface{}{
					"Environment": []string{"TEST=TEST"},
				}).Run(session)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}

				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				return true
			},
			timeout: 1 * time.Second,
		},
		{
			name: "Create another service",
			run: func(t *testing.T) bool {
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				return true
			},
			timeout: 1 * time.Second,
		},
		{
			name: "Stop services",
			run: func(t *testing.T) bool {
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				return true
			},
			timeout: 1 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.run(t))
			assert.True(t, tt.wait(t, tt.timeout))
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
