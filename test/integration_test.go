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
				done <- false
				close(done)
				return
			case b := <-recv:
				done <- b
				close(done)
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
			timeout: 20 * time.Second,
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
			timeout: 20 * time.Second,
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
					targetService                          string
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				startDocker := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							if e.Action != "create" {
								break
							}
							if v, ok := e.Actor.Attributes["name"]; ok {
								if v != "TestPlugin" {
									break
								}
							} else {
								break
							}
							targetService = e.Actor.ID
							return true
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				var startDockerVerify <-chan bool

				startDB := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							if _, ok := d["new_val"]; !ok {
								break
							}
							if d["new_val"].(map[string]interface{})["ServiceName"].(string) != "TestPlugin" {
								break
							}
							if d["new_val"].(map[string]interface{})["State"].(string) != "Active" {
								break
							}
							return true
						default:
							break
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-startDocker
						<-startDockerVerify
						<-startDB
						log.Printf("Done (main)")
						break L
					case v := <-startDocker:
						if v {
							log.Printf("Setting foundService to %v", v)
							startDockerVerify = timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
								dc, err := client.NewEnvClient()
								if err != nil {
									t.Errorf("%v", err)
									return false
								}
								cntxt := args[0].(context.Context)

								for {
									time.Sleep(100 * time.Millisecond)
									select {
									case <-cntxt.Done():
										return false
									default:
										break
									}

									inspect, _, err := dc.ServiceInspectWithRaw(cntxt, targetService)
									if err != nil {
										log.Printf("inspect error: %v", err)
										return false
									}
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
									return true
								}
							})
							foundService = v
						}
					case v := <-startDockerVerify:
						if v {
							log.Printf("Setting serviceVerify to %v", v)
							serviceVerify = v
						}
					case v := <-startDB:
						if v {
							log.Printf("Setting dbUpdated to %v", v)
							dbUpdated = v
						}
					default:
						break
					}
					if foundService && serviceVerify && dbUpdated {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !foundService {
					t.Errorf("Docker start event not detected")
				}
				if !serviceVerify {
					t.Errorf("Database start event not verified with params")
				}
				if !dbUpdated {
					t.Errorf("Docker start db not updated")
				}

				return foundService && serviceVerify && dbUpdated

			},
			timeout: 60 * time.Second,
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
					rD, rDB, rDu, rDBu bool = false, false, false, false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

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
				restartDB := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							if _, ok := d["new_val"]; !ok {
								break
							}
							if d["new_val"].(map[string]interface{})["ServiceName"].(string) != "TestPlugin" {
								break
							}
							if d["new_val"].(map[string]interface{})["State"].(string) != "Restarting" {
								break
							}
							return true
						default:
							break
						}
						time.Sleep(100 * time.Millisecond)
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
						time.Sleep(100 * time.Millisecond)
					}
				})
				var restartedDBUpdated = make(<-chan bool)

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-restartDocker
						<-restartDB
						<-restartedDockerUpdated
						<-restartedDBUpdated
						log.Printf("Done (main)")
						break L
					case v := <-restartDocker:
						if v {
							log.Printf("Setting rD to %v", v)
							rD = v
						}
					case v := <-restartDB:
						if v {
							log.Printf("Setting rDB to %v", v)
							restartedDBUpdated = timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
										if _, ok := d["new_val"]; !ok {
											break
										}
										if d["new_val"].(map[string]interface{})["ServiceName"].(string) != "TestPlugin" {
											break
										}
										if d["new_val"].(map[string]interface{})["State"].(string) != "Active" {
											break
										}
										return true
									default:
										break
									}
									time.Sleep(100 * time.Millisecond)
								}
							})
							rDB = v
						}
					case v := <-restartedDockerUpdated:
						if v {
							log.Printf("Setting rDu to %v", v)
							rDu = v
						}
					case v := <-restartedDBUpdated:
						if v {
							log.Printf("Setting rDBu to %v", v)
							rDBu = v
						}
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
			timeout: 60 * time.Second,
		},
		{
			name: "Create another service",
			run: func(t *testing.T) bool {
				_, err := r.DB("Controller").Table("Plugins").Insert(map[string]interface{}{
					"Name":          "Harness",
					"ServiceID":     "",
					"ServiceName":   "TestPlugin2",
					"DesiredState":  "Activate",
					"State":         "Available",
					"Address":       "",
					"ExternalPorts": []string{"6000/tcp"},
					"InternalPorts": []string{"6000/tcp"},
					"OS":            string(rethink.PluginOSPosix),
					"Environment":   []string{"WHATEVER=WHATEVER"},
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
					targetService                          string
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				startDocker := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							if e.Action != "create" {
								break
							}
							if v, ok := e.Actor.Attributes["name"]; ok {
								if v != "TestPlugin2" {
									break
								}
							} else {
								break
							}
							targetService = e.Actor.ID
							return true
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				var startDockerVerify <-chan bool

				startDB := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							if v, ok := d["new_val"]; !ok {
								break
							} else {
								log.Printf("change doc: %+v", v)
							}
							if d["new_val"].(map[string]interface{})["ServiceName"].(string) != "TestPlugin2" {
								break
							}
							if d["new_val"].(map[string]interface{})["State"].(string) != "Active" {
								break
							}
							return true
						default:
							break
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-startDocker
						<-startDockerVerify
						<-startDB
						log.Printf("Done (main)")
						break L
					case v := <-startDocker:
						if v {
							log.Printf("Setting foundService to %v", v)
							startDockerVerify = timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
								dc, err := client.NewEnvClient()
								if err != nil {
									t.Errorf("%v", err)
									return false
								}
								cntxt := args[0].(context.Context)

								for {
									time.Sleep(100 * time.Millisecond)
									select {
									case <-cntxt.Done():
										return false
									default:
										break
									}

									inspect, _, err := dc.ServiceInspectWithRaw(cntxt, targetService)
									if err != nil {
										log.Printf("inspect error: %v", err)
										return false
									}
									if *inspect.Spec.Mode.Replicated.Replicas != uint64(1) {
										continue
									}
									if len(inspect.Endpoint.Ports) < 1 {
										continue
									}
									if inspect.Endpoint.Ports[0].Protocol != swarm.PortConfigProtocolTCP {
										continue
									}
									if inspect.Endpoint.Ports[0].PublishedPort != uint32(6000) {
										continue
									}
									if inspect.Endpoint.Ports[0].TargetPort != uint32(6000) {
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
									for _, env := range inspect.Spec.TaskTemplate.ContainerSpec.Env {
										if env == "WHATEVER=WHATEVER" {
											return true
										}
									}
								}
							})
							foundService = v
						}
					case v := <-startDockerVerify:
						if v {
							log.Printf("Setting serviceVerify to %v", v)
							serviceVerify = v
						}
					case v := <-startDB:
						if v {
							log.Printf("Setting dbUpdated to %v", v)
							dbUpdated = v
						}
					default:
						break
					}
					if foundService && serviceVerify && dbUpdated {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !foundService {
					t.Errorf("Docker start event not detected")
				}
				if !serviceVerify {
					t.Errorf("Docker start event not verified with params")
				}
				if !dbUpdated {
					t.Errorf("Docker start db not updated")
				}

				return foundService && serviceVerify && dbUpdated

			},
			timeout: 60 * time.Second,
		},
		{
			name: "Stop services",
			run: func(t *testing.T) bool {
				filter := make(map[string]string)
				update := make(map[string]string)
				filter["ServiceName"] = "TestPlugin"
				update["DesiredState"] = "Stop"
				_, err := r.DB("Controller").Table("Plugins").Filter(filter).Update(update).RunWrite(session)
				if err != nil {
					t.Errorf("db error: %v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					dockerStopped, dbStopped bool = false, false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				stopDocker := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							if e.Action != "remove" {
								break
							}
							if v, ok := e.Actor.Attributes["name"]; ok {
								if v != "TestPlugin" {
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

				stopDB := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							if v, ok := d["new_val"]; !ok {
								break
							} else {
								log.Printf("change doc: %+v", v)
							}
							if d["new_val"].(map[string]interface{})["ServiceName"].(string) != "TestPlugin" {
								break
							}
							if d["new_val"].(map[string]interface{})["State"].(string) != "Stopped" {
								break
							}
							return true
						default:
							break
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-stopDocker
						<-stopDB
						log.Printf("Done (main)")
						break L
					case v := <-stopDocker:
						if v {
							log.Printf("Setting dockerStopped to %v", v)
							dockerStopped = v
						}
					case v := <-stopDB:
						if v {
							log.Printf("Setting dbStopped to %v", v)
							dbStopped = v
						}
					default:
						break
					}
					if dockerStopped && dbStopped {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !dockerStopped {
					t.Errorf("Docker stop event not detected")
				}
				if !dbStopped {
					t.Errorf("Database stop event not detected")
				}

				return dockerStopped && dbStopped
			},
			timeout: 60 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := make(chan bool)
			go func() {
				res <- tt.wait(t, tt.timeout)
				close(res)
				return
			}()
			time.Sleep(3 * time.Second)
			assert.True(t, tt.run(t))
			assert.True(t, <-res)
			time.Sleep(3 * time.Second)
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
