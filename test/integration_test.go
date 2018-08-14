package test

import (
	"context"
	"fmt"
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

func dumpEverything(ctx context.Context, t *testing.T, dockerClient *client.Client, session *r.Session) {
	var doc map[string]interface{}

	t.Errorf("Dumping services...")
	services, _ := dockerClient.ServiceList(ctx, types.ServiceListOptions{})
	for _, service := range services {
		t.Errorf("Service %v: %+v", service.Spec.Annotations.Name, service)
		t.Errorf("Replicas: %v", *service.Spec.Mode.Replicated.Replicas)
	}

	t.Errorf("Dumping ports table...")
	cursor, _ := r.DB("Controller").Table("Ports").Run(session)
	for cursor.Next(&doc) {
		t.Errorf("Port entry: %+v", doc)
	}

	t.Errorf("Dumping plugins table...")
	cursor, _ = r.DB("Controller").Table("Plugins").Run(session)
	for cursor.Next(&doc) {
		t.Errorf("Plugin entry: %+v", doc)
	}
}

func timoutTester(ctx context.Context, args []interface{}, f func(args ...interface{}) bool) <-chan bool {
	done := make(chan bool)

	go func() {
		for {
			recv := make(chan bool)

			go func() {
				recv <- f(args...)
				close(recv)
				return
			}()

			select {
			case <-ctx.Done():
				log.Printf("Context done (wrapper), closing channels")
				done <- false
				close(done)
				log.Printf("Context done (wrapper), sent and closed channels")
				return
			case b := <-recv:
				log.Printf("Received %v from test, sending and closing channels", b)
				done <- b
				close(done)
				log.Printf("Received (wrapper), sent and closed channels")
				return
			}
		}
	}()

	return done
}

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
						log.Printf("db err: %v", err)
						continue
					}

					for cursor.Next(&doc) {
						// log.Printf("Port DB entry: %+v", doc)
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
				dumpEverything(ctx, t, dockerClient, session)
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
						log.Printf("db err: %v", err)
						continue
					}

					// Get all plugins from the db (should only be one)
					var pluginEntries []map[string]interface{}
					var doc map[string]interface{}

					for cursor.Next(&doc) {
						// log.Printf("Plugin DB entry: %+v", doc)
						pluginEntries = append(pluginEntries, doc)
					}

					if len(pluginEntries) < 1 {
						time.Sleep(time.Second)
						continue
					}

					if len(pluginEntries) != 1 {
						t.Errorf("shouldn't be %v plugins", len(pluginEntries))
						break
					}

					assert.Equal(t, "Harness", pluginEntries[0]["Name"].(string))
					assert.Equal(t, "", pluginEntries[0]["ServiceID"].(string))
					assert.Equal(t, "", pluginEntries[0]["ServiceName"].(string))
					assert.Equal(t, string(rethink.DesiredStateNull), pluginEntries[0]["DesiredState"].(string))
					assert.Equal(t, string(rethink.StateAvailable), pluginEntries[0]["State"].(string))
					assert.Equal(t, "", pluginEntries[0]["Address"].(string))
					assert.Equal(t, string(rethink.PluginOSAll), pluginEntries[0]["OS"].(string))
					return true
				}
				dumpEverything(ctx, t, dockerClient, session)
				return false
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
							// log.Printf("Found service by ID %v", service.ID)
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
					// log.Printf("Verified service %+v", inspect)
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
							// log.Printf("empty id")
							break
						}
						if doc["State"].(string) != "Active" {
							// log.Printf("bad state: %v", doc["State"])
							break
						}
						if doc["DesiredState"].(string) != "" {
							// log.Printf("bad desiredstate: %v", doc["DesiredState"])
							break
						}
						// log.Printf("DB update verified %+v", doc)
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
					"DesiredState": "Restart", "Environment": []string{"TEST=TEST"},
				}).Run(session)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}

				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					rD, rDB, rDu, rDBu bool = false, true, true, true
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
				log.Printf("starting timeout tests")

				// Create goroutine for each condition we want to satisfy
				// and pass same parent context
				// goroutines return <-chan bool
				restartDocker := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
					dc, err := client.NewEnvClient()
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					cntxt := args[0].(context.Context)
					eventChan, errChan := dc.Events(cntxt, types.EventsOptions{})

					for {
						select {
						case <-cntxt.Done():
							log.Printf("Done (routine)")
							return false
						case e := <-errChan:
							log.Println(fmt.Errorf("%v", e))
							return false
						case e := <-eventChan:
							if e.Type != "service" {
								break
							}
							log.Printf("service event received: %+v", e)
							if e.Action != "update" {
								break
							}
							if v, ok := e.Actor.Attributes["name"]; ok {
								if v != "TestPlugin" {
									break
								}
							} else {
								break
							}
							if v, ok := e.Actor.Attributes["updatestate.new"]; ok {
								if v != "updating" {
									break
								}
							} else {
								break
							}
							return true
						}
						time.Sleep(100 * time.Millisecond)
					}
				})
				/*restartDB := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
					sessionTest, err := r.Connect(r.ConnectOpts{
						Address: "127.0.0.1",
					})
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					cntxt := args[0].(context.Context)
					changeChan, errChan := func(s *r.Session) (<-chan map[string]interface{}, <-chan error) {
						changes := make(chan map[string]interface{})
						errs := make(chan error)
						go func() {
							cursor, err := r.DB("Controller").Table("Plugins").Changes().Run(s)
							if err != nil {
								log.Println(fmt.Errorf("%v", err))
								errs <- err
							}
							var doc map[string]interface{}
							for cursor.Next(&doc) {
								changes <- doc
							}
						}()
						return changes, errs
					}(sessionTest)

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-errChan:
							log.Println(fmt.Errorf("%v", e))
							return false
						case d := <-changeChan:
							if d["ServiceName"] != "TestPlugin" {
								continue
							}
							if d["State"] != "Restarting" {
								continue
							}
							return true
						default:
							continue
						}
					}
				})
				restartedDockerUpdated := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
					dc, err := client.NewEnvClient()
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					cntxt := args[0].(context.Context)
					eventChan, errChan := dc.Events(cntxt, types.EventsOptions{})
					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-errChan:
							log.Println(fmt.Errorf("%v", e))
							return false
						case e := <-eventChan:
							if e.Type != "service" {
								break
							}
							if e.Action != "update" {
								break
							}
							if v, ok := e.Actor.Attributes["name"]; ok {
								if v != "TestPlugin" {
									break
								}
							} else {
								break
							}
							if v, ok := e.Actor.Attributes["updatestate.new"]; ok {
								if v != "completed" {
									break
								}
							} else {
								break
							}
							return true
						}
						time.Sleep(time.Second)
					}
				})
				restartedDBUpdated := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
					sessionTest, err := r.Connect(r.ConnectOpts{
						Address: "127.0.0.1",
					})
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					cntxt := args[0].(context.Context)
					changeChan, errChan := func(s *r.Session) (<-chan map[string]interface{}, <-chan error) {
						changes := make(chan map[string]interface{})
						errs := make(chan error)
						go func() {
							cursor, err := r.DB("Controller").Table("Plugins").Changes().Run(s)
							if err != nil {
								log.Println(fmt.Errorf("%v", err))
								errs <- err
							}
							var doc map[string]interface{}
							for cursor.Next(&doc) {
								changes <- doc
							}
						}()
						return changes, errs
					}(sessionTest)

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-errChan:
							log.Println(fmt.Errorf("%v", e))
							return false
						case d := <-changeChan:
							if d["ServiceName"] != "TestPlugin" {
								continue
							}
							if d["State"] != "Restarting" {
								continue
							}
							return true
						default:
							continue
						}
					}
				})*/

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						log.Printf("Done (main)")
						rD = <-restartDocker
						log.Printf("Done - received %v(main)", rD)
						/*<-restartDB
						<-restartedDockerUpdated
						<-restartedDBUpdated*/
						break L
					case v := <-restartDocker:
						log.Printf("Setting rD to %v", v)
						rD = v
					/*case v := <-restartDB:
						rDB = v
						break
					case v := <-restartedDockerUpdated:
						rDu = v
						break
					case v := <-restartedDBUpdated:
						rDBu = v
						break*/
					default:
						break
					}
					if rD && rDB && rDu && rDBu {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !rD {
					t.Errorf("Docker restart event not detected")
				}
				if !rDB {
					t.Errorf("Database restart event not detected")
				}
				if !rDu {
					t.Errorf("Docker restart complete event not detected")
				}
				if !rDBu {
					t.Errorf("Database restart complete event not detected")
				}

				return rD && rDB && rDu && rDBu
			},
			timeout: 10 * time.Second,
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
			res := make(chan bool)
			go func() {
				res <- tt.wait(t, tt.timeout)
				return
			}()
			assert.True(t, tt.run(t))
			assert.True(t, <-res)
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
