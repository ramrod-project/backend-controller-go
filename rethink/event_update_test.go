package rethink

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	events "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/helper"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
)

var testPluginService = swarm.ServiceSpec{
	Annotations: swarm.Annotations{
		Name: "Harness-1080tcp",
		Labels: map[string]string{
			"os": "posix",
		},
	},
	TaskTemplate: swarm.TaskSpec{
		ContainerSpec: swarm.ContainerSpec{
			Env: []string{
				"PLUGIN=Harness",
				"PORT=1080",
				"LOGLEVEL=DEBUG",
				"STAGE=DEV",
				"PLUGIN_NAME=Harness-1080tcp",
			},
			Image: "ramrodpcp/interpreter-plugin:" + getTagFromEnv(),
		},
		RestartPolicy: &swarm.RestartPolicy{
			Condition: "on-failure",
		},
		Networks: []swarm.NetworkAttachmentConfig{
			swarm.NetworkAttachmentConfig{
				Target: "pcp",
			},
		},
	},
	UpdateConfig: &swarm.UpdateConfig{
		Parallelism: 0,
		Delay:       0,
	},
	EndpointSpec: &swarm.EndpointSpec{
		Mode: swarm.ResolutionModeVIP,
		Ports: []swarm.PortConfig{
			swarm.PortConfig{
				Protocol:      swarm.PortConfigProtocolTCP,
				PublishedPort: 1080,
				TargetPort:    1080,
				PublishMode:   swarm.PortConfigPublishModeHost,
			},
		},
	},
}

var testPluginServiceWin = swarm.ServiceSpec{
	Annotations: swarm.Annotations{
		Name: "Harness-2080tcp",
		Labels: map[string]string{
			"os": "nt",
		},
	},
	TaskTemplate: swarm.TaskSpec{
		ContainerSpec: swarm.ContainerSpec{
			Env: []string{
				"PLUGIN=Harness",
				"PORT=2080",
				"LOGLEVEL=DEBUG",
				"STAGE=DEV",
				"PLUGIN_NAME=Harness-2080tcp",
			},
			Image: "ramrodpcp/interpreter-plugin:" + getTagFromEnv(),
		},
		RestartPolicy: &swarm.RestartPolicy{
			Condition: "on-failure",
		},
		Networks: []swarm.NetworkAttachmentConfig{
			swarm.NetworkAttachmentConfig{
				Target: "pcp",
			},
		},
	},
	UpdateConfig: &swarm.UpdateConfig{
		Parallelism: 0,
		Delay:       0,
	},
	EndpointSpec: &swarm.EndpointSpec{
		Mode: swarm.ResolutionModeVIP,
		Ports: []swarm.PortConfig{
			swarm.PortConfig{
				Protocol:      swarm.PortConfigProtocolTCP,
				PublishedPort: 2080,
				TargetPort:    2080,
				PublishMode:   swarm.PortConfigPublishModeHost,
			},
		},
	},
}

var testPluginServiceExtra = swarm.ServiceSpec{
	Annotations: swarm.Annotations{
		Name: "Harness-3080udp",
		Labels: map[string]string{
			"os": "posix",
		},
	},
	TaskTemplate: swarm.TaskSpec{
		ContainerSpec: swarm.ContainerSpec{
			Env: []string{
				"PLUGIN=Harness",
				"PORT=3080",
				"LOGLEVEL=DEBUG",
				"STAGE=DEV",
				"PLUGIN_NAME=Harness-3080udp",
			},
			Image: "ramrodpcp/interpreter-plugin-extra:" + getTagFromEnv(),
		},
		RestartPolicy: &swarm.RestartPolicy{
			Condition: "on-failure",
		},
		Networks: []swarm.NetworkAttachmentConfig{
			swarm.NetworkAttachmentConfig{
				Target: "pcp",
			},
		},
	},
	UpdateConfig: &swarm.UpdateConfig{
		Parallelism: 0,
		Delay:       0,
	},
	EndpointSpec: &swarm.EndpointSpec{
		Mode: swarm.ResolutionModeVIP,
		Ports: []swarm.PortConfig{
			swarm.PortConfig{
				Protocol:      swarm.PortConfigProtocolUDP,
				PublishedPort: 3080,
				TargetPort:    3080,
				PublishMode:   swarm.PortConfigPublishModeHost,
			},
		},
	},
}

func getTagFromEnv() string {
	temp := os.Getenv("TAG")
	if temp == "" {
		temp = "latest"
	}
	return temp
}

func dbPluginChanges(s *r.Session) (<-chan map[string]interface{}, <-chan error) {
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
}

// Takes a (1) context and a (2) condition checking function argument
func dockerMonitor(args ...interface{}) bool {
	dc, err := client.NewEnvClient()
	if err != nil {
		return false
	}
	// Get expected arguments
	cntxt := args[0].(context.Context)
	condition := args[1].(func(events.Message) bool)

	filter := filters.NewArgs()
	filter.Add("type", "container")
	filter.Add("type", "service")
	eventChan, errChan := dc.Events(cntxt, types.EventsOptions{
		Filters: filter,
	})

	for {
		select {
		case <-cntxt.Done():
			return false
		case e := <-errChan:
			log.Println(fmt.Errorf("%v", e))
			return false
		case e := <-eventChan:
			if condition(e) {
				return true
			}
			break
		default:
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Takes a (1) context and a (2) condition checking function argument
func dbMonitor(args ...interface{}) bool {
	sessionTest, err := r.Connect(r.ConnectOpts{
		Address: "127.0.0.1",
	})
	if err != nil {
		return false
	}
	cntxt := args[0].(context.Context)
	condition := args[1].(func(map[string]interface{}) bool)
	changeChan, errChan := dbPluginChanges(sessionTest)

	for {
		select {
		case <-cntxt.Done():
			return false
		case e := <-errChan:
			log.Println(fmt.Errorf("%v", e))
			return false
		case d := <-changeChan:
			if condition(d) {
				return true
			}
			break
		default:
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestEventUpdate(t *testing.T) {
	oldStage := os.Getenv("STAGE")
	os.Setenv("STAGE", "TESTING")

	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// Set up clean environment
	if err := test.DockerCleanUp(ctx, dockerClient, ""); err != nil {
		t.Errorf("setup error: %v", err)
	}

	netID, err := test.CheckCreateNet("pcp")
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	var eventBrainSpec = test.BrainSpec
	eventBrainSpec.Networks = []swarm.NetworkAttachmentConfig{
		swarm.NetworkAttachmentConfig{
			Target:  "pcp",
			Aliases: []string{"rethinkdb"},
		},
	}
	session, brainID, err := test.StartBrain(ctx, t, dockerClient, eventBrainSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	_, err = r.DB("Controller").Table("Plugins").Insert(map[string]interface{}{
		"Name":          "Harness",
		"ServiceID":     "",
		"ServiceName":   "Harness-1080tcp",
		"DesiredState":  "Activate",
		"State":         "Available",
		"Interface":     "192.168.1.1",
		"ExternalPorts": []string{"1080/tcp"},
		"InternalPorts": []string{"1080/tcp"},
		"OS":            string(PluginOSAll),
	}).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	_, err = r.DB("Controller").Table("Plugins").Insert(map[string]interface{}{
		"Name":          "Harness",
		"ServiceID":     "",
		"ServiceName":   "Harness-2080tcp",
		"DesiredState":  "Activate",
		"State":         "Available",
		"Interface":     "192.168.1.2",
		"ExternalPorts": []string{"2080/tcp"},
		"InternalPorts": []string{"2080/tcp"},
		"OS":            string(PluginOSWindows),
	}).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	_, err = r.DB("Controller").Table("Plugins").Insert(map[string]interface{}{
		"Name":          "Harness",
		"ServiceID":     "",
		"ServiceName":   "Harness-3080udp",
		"DesiredState":  "Activate",
		"State":         "Available",
		"Interface":     "192.168.1.2",
		"ExternalPorts": []string{"3080/udp"},
		"InternalPorts": []string{"3080/udp"},
		"OS":            string(PluginOSPosix),
		"Extra":         true,
	}).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	var (
		targetService    string
		targetServiceWin string
	)

	tests := []struct {
		name    string
		run     func(t *testing.T) bool
		wait    func(t *testing.T, timeout time.Duration) bool
		timeout time.Duration
	}{
		{
			name: "service start",
			run: func(t *testing.T) bool {
				targetService, err = test.StartIntegrationTestService(ctx, dockerClient, testPluginService)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					startedService = false
					dbUpdated      = false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				startDocker := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(e events.Message) bool {
						if e.Type != "container" {
							return false
						}
						if e.Action != "health_status: healthy" && e.Status != "health_status: healthy" {
							return false
						}
						if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
							if v != "Harness-1080tcp" {
								return false
							}
						} else {
							return false
						}
						return true
					},
				}, dockerMonitor)

				startDB := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(d map[string]interface{}) bool {
						var c map[string]interface{}
						if v, ok := d["new_val"]; !ok {
							return false
						} else {
							c = v.(map[string]interface{})
						}
						if c["ServiceName"].(string) != "Harness-1080tcp" {
							return false
						}
						if c["DesiredState"].(string) != "" {
							return false
						}
						if c["State"].(string) != "Active" {
							return false
						}
						if c["ServiceID"].(string) == "" {
							return false
						}
						return true
					},
				}, dbMonitor)

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						break L
					case v := <-startDocker:
						if v {
							log.Printf("Setting startedService to %v", v)
							startedService = v
						}
					case v := <-startDB:
						if v {
							log.Printf("Setting dbUpdated to %v", v)
							dbUpdated = v
						}
					default:
						break
					}
					if startedService && dbUpdated {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !startedService {
					t.Errorf("Service start not detected in Docker")
				}
				if !dbUpdated {
					t.Errorf("DB not updated with service start info.")
				}

				return startedService && dbUpdated
			},
			timeout: 30 * time.Second,
		},
		{
			name: "service start extra",
			run: func(t *testing.T) bool {
				_, err = test.StartIntegrationTestService(ctx, dockerClient, testPluginServiceExtra)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					startedService = false
					dbUpdated      = false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				startDocker := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(e events.Message) bool {
						if e.Type != "container" {
							return false
						}
						if e.Action != "health_status: healthy" && e.Status != "health_status: healthy" {
							return false
						}
						if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
							if v != "Harness-3080udp" {
								return false
							}
						} else {
							return false
						}
						return true
					},
				}, dockerMonitor)

				startDB := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(d map[string]interface{}) bool {
						var c map[string]interface{}
						if v, ok := d["new_val"]; !ok {
							return false
						} else {
							c = v.(map[string]interface{})
						}
						if c["ServiceName"].(string) != "Harness-3080udp" {
							return false
						}
						if c["DesiredState"].(string) != "" {
							return false
						}
						if c["State"].(string) != "Active" {
							return false
						}
						if c["ServiceID"].(string) == "" {
							return false
						}
						return true
					},
				}, dbMonitor)

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						break L
					case v := <-startDocker:
						if v {
							log.Printf("Setting startedService to %v", v)
							startedService = v
						}
					case v := <-startDB:
						if v {
							log.Printf("Setting dbUpdated to %v", v)
							dbUpdated = v
						}
					default:
						break
					}
					if startedService && dbUpdated {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !startedService {
					t.Errorf("Service start not detected in Docker")
				}
				if !dbUpdated {
					t.Errorf("DB not updated with service start info.")
				}

				return startedService && dbUpdated
			},
			timeout: 30 * time.Second,
		},
		{
			name: "service update",
			run: func(t *testing.T) bool {
				newTestPluginService := testPluginService
				newTestPluginService.TaskTemplate.ContainerSpec.Env = append(newTestPluginService.TaskTemplate.ContainerSpec.Env, "TEST=TEST2")

				insp, _, err := dockerClient.ServiceInspectWithRaw(ctx, targetService)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				dockerClient.ServiceUpdate(ctx, targetService, insp.Meta.Version, newTestPluginService, types.ServiceUpdateOptions{})
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					serviceUpdating = false
					dbUpdating      = false
					serviceUpdated  = false
					dbUpdated       = false
					updatedDB       = make(<-chan bool)
					updatedDocker   = make(<-chan bool)
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				updatingDocker := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(e events.Message) bool {
						if e.Type != "service" {
							return false
						}
						if e.Action != "update" {
							return false
						}
						if v, ok := e.Actor.Attributes["name"]; ok {
							if v != "Harness-1080tcp" {
								return false
							}
						} else {
							return false
						}
						if v, ok := e.Actor.Attributes["updatestate.new"]; ok {
							if v != "updating" {
								return false
							}
						} else {
							return false
						}
						return true
					},
				}, dockerMonitor)

				updatingDB := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(d map[string]interface{}) bool {
						var c map[string]interface{}
						if v, ok := d["new_val"]; !ok {
							return false
						} else {
							c = v.(map[string]interface{})
						}
						if c["ServiceName"].(string) != "Harness-1080tcp" {
							return false
						}
						if c["DesiredState"].(string) != "" {
							return false
						}
						if c["State"].(string) != "Restarting" {
							return false
						}
						if c["ServiceID"].(string) == "" {
							return false
						}
						return true
					},
				}, dbMonitor)

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						break L
					case v := <-updatingDocker:
						if v {
							log.Printf("Setting serviceUpdating to %v", v)
							serviceUpdating = v
							updatedDocker = helper.TimeoutTester(timeoutCtx, []interface{}{
								timeoutCtx,
								func(e events.Message) bool {
									if e.Type != "container" {
										return false
									}
									if e.Action != "health_status: healthy" && e.Status != "health_status: healthy" {
										return false
									}
									if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
										if v != "Harness-1080tcp" {
											return false
										}
									} else {
										return false
									}
									return true
								},
							}, dockerMonitor)
						}
					case v := <-updatingDB:
						if v {
							log.Printf("Setting dbUpdating to %v", v)
							dbUpdating = v
							updatedDB = helper.TimeoutTester(timeoutCtx, []interface{}{
								timeoutCtx,
								func(d map[string]interface{}) bool {
									var c map[string]interface{}
									if v, ok := d["new_val"]; !ok {
										return false
									} else {
										c = v.(map[string]interface{})
									}
									if c["ServiceName"].(string) != "Harness-1080tcp" {
										return false
									}
									if c["DesiredState"].(string) != "" {
										return false
									}
									if c["State"].(string) != "Active" {
										return false
									}
									if c["ServiceID"].(string) == "" {
										return false
									}
									return true
								},
							}, dbMonitor)
						}
					case v := <-updatedDocker:
						if v {
							log.Printf("Setting serviceUpdated to %v", v)
							serviceUpdated = v
						}
					case v := <-updatedDB:
						if v {
							log.Printf("Setting dbUpdated to %v", v)
							dbUpdated = v
						}
					default:
						break
					}
					if serviceUpdating && dbUpdating && serviceUpdated && dbUpdated {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !serviceUpdating {
					t.Errorf("service updating not detected in Docker")
				}
				if !dbUpdating {
					t.Errorf("DB not updated with service updating info")
				}
				if !serviceUpdated {
					t.Errorf("service update complete not detected in Docker")
				}
				if !dbUpdated {
					t.Errorf("DB not updated with service update complete info")
				}

				return serviceUpdating && dbUpdating && serviceUpdated && dbUpdated
			},
			timeout: 30 * time.Second,
		},
		{
			name: "service stop",
			run: func(t *testing.T) bool {
				err = dockerClient.ServiceRemove(ctx, targetService)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					dockerStopped = false
					dbStopped     = false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				stopDocker := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(e events.Message) bool {
						if e.Type != "container" {
							return false
						}
						if e.Action != "die" {
							return false
						}
						if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
							if v != "Harness-1080tcp" {
								return false
							}
						} else {
							return false
						}
						return true
					},
				}, dockerMonitor)

				stopDB := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(d map[string]interface{}) bool {
						var c map[string]interface{}
						if v, ok := d["new_val"]; !ok {
							return false
						} else {
							c = v.(map[string]interface{})
						}
						if c["ServiceName"].(string) != "Harness-1080tcp" {
							return false
						}
						if c["DesiredState"].(string) != "" {
							return false
						}
						if c["State"].(string) != "Stopped" {
							return false
						}
						if c["ServiceID"].(string) == "" {
							return false
						}
						return true
					},
				}, dbMonitor)

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
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
					t.Errorf("service stop not detected in Docker")
				}
				if !dbStopped {
					t.Errorf("db not updated with service stop info")
				}

				return dockerStopped && dbStopped
			},
			timeout: 30 * time.Second,
		},
		{
			name: "service start (windows)",
			run: func(t *testing.T) bool {
				targetServiceWin, err = test.StartIntegrationTestService(ctx, dockerClient, testPluginServiceWin)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					startedService = false
					dbUpdated      = false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				startDocker := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(e events.Message) bool {
						if e.Type != "service" {
							return false
						}
						if e.Action != "create" {
							return false
						}
						if v, ok := e.Actor.Attributes["name"]; ok {
							if v != "Harness-2080tcp" {
								return false
							}
						} else {
							return false
						}
						return true
					},
				}, dockerMonitor)

				startDB := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(d map[string]interface{}) bool {
						var c map[string]interface{}
						if v, ok := d["new_val"]; !ok {
							return false
						} else {
							c = v.(map[string]interface{})
						}
						if c["ServiceName"].(string) != "Harness-2080tcp" {
							return false
						}
						if c["DesiredState"].(string) != "" {
							return false
						}
						if c["State"].(string) != "Active" {
							return false
						}
						if c["ServiceID"].(string) == "" {
							return false
						}
						return true
					},
				}, dbMonitor)

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						break L
					case v := <-startDocker:
						if v {
							log.Printf("Setting startedService to %v", v)
							startedService = v
						}
					case v := <-startDB:
						if v {
							log.Printf("Setting dbUpdated to %v", v)
							dbUpdated = v
						}
					default:
						break
					}
					if startedService && dbUpdated {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !startedService {
					t.Errorf("Service start not detected in Docker")
				}
				if !dbUpdated {
					t.Errorf("DB not updated with service start info.")
				}

				return startedService && dbUpdated
			},
			timeout: 30 * time.Second,
		},
		{
			name: "service update (windows)",
			run: func(t *testing.T) bool {
				newTestPluginServiceWin := testPluginServiceWin
				newTestPluginServiceWin.TaskTemplate.ContainerSpec.Env = append(newTestPluginServiceWin.TaskTemplate.ContainerSpec.Env, "TEST=TEST2")

				insp, _, err := dockerClient.ServiceInspectWithRaw(ctx, targetServiceWin)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				dockerClient.ServiceUpdate(ctx, targetServiceWin, insp.Meta.Version, newTestPluginServiceWin, types.ServiceUpdateOptions{})
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					serviceUpdating = false
					dbUpdating      = false
					serviceUpdated  = false
					dbUpdated       = false
					updatedDB       = make(<-chan bool)
					updatedDocker   = make(<-chan bool)
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				updatingDocker := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(e events.Message) bool {
						if e.Type != "service" {
							return false
						}
						if e.Action != "update" {
							return false
						}
						if v, ok := e.Actor.Attributes["name"]; ok {
							if v != "Harness-2080tcp" {
								return false
							}
						} else {
							return false
						}
						if v, ok := e.Actor.Attributes["updatestate.new"]; ok {
							if v != "updating" {
								return false
							}
						} else {
							return false
						}
						return true
					},
				}, dockerMonitor)

				updatingDB := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(d map[string]interface{}) bool {
						var c map[string]interface{}
						if v, ok := d["new_val"]; !ok {
							return false
						} else {
							c = v.(map[string]interface{})
						}
						if c["ServiceName"].(string) != "Harness-2080tcp" {
							return false
						}
						if c["DesiredState"].(string) != "" {
							return false
						}
						if c["State"].(string) != "Restarting" {
							return false
						}
						if c["ServiceID"].(string) == "" {
							return false
						}
						return true
					},
				}, dbMonitor)

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						break L
					case v := <-updatingDocker:
						if v {
							log.Printf("Setting serviceUpdating to %v", v)
							serviceUpdating = v
							updatedDocker = helper.TimeoutTester(timeoutCtx, []interface{}{
								timeoutCtx,
								func(e events.Message) bool {
									if e.Type != "service" {
										return false
									}
									if e.Action != "update" {
										return false
									}
									if v, ok := e.Actor.Attributes["name"]; ok {
										if v != "Harness-2080tcp" {
											return false
										}
									} else {
										return false
									}
									if v, ok := e.Actor.Attributes["updatestate.new"]; ok {
										if v != "completed" {
											return false
										}
									} else {
										return false
									}
									return true
								},
							}, dockerMonitor)
						}
					case v := <-updatingDB:
						if v {
							log.Printf("Setting dbUpdating to %v", v)
							dbUpdating = v
							updatedDB = helper.TimeoutTester(timeoutCtx, []interface{}{
								timeoutCtx,
								func(d map[string]interface{}) bool {
									var c map[string]interface{}
									if v, ok := d["new_val"]; !ok {
										return false
									} else {
										c = v.(map[string]interface{})
									}
									if c["ServiceName"].(string) != "Harness-2080tcp" {
										return false
									}
									if c["DesiredState"].(string) != "" {
										return false
									}
									if c["State"].(string) != "Active" {
										return false
									}
									if c["ServiceID"].(string) == "" {
										return false
									}
									return true
								},
							}, dbMonitor)
						}
					case v := <-updatedDocker:
						if v {
							log.Printf("Setting serviceUpdated to %v", v)
							serviceUpdated = v
						}
					case v := <-updatedDB:
						if v {
							log.Printf("Setting dbUpdated to %v", v)
							dbUpdated = v
						}
					default:
						break
					}
					if serviceUpdating && dbUpdating && serviceUpdated && dbUpdated {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !serviceUpdating {
					t.Errorf("service updating not detected in Docker")
				}
				if !dbUpdating {
					t.Errorf("DB not updated with service updating info")
				}
				if !serviceUpdated {
					t.Errorf("service update complete not detected in Docker")
				}
				if !dbUpdated {
					t.Errorf("DB not updated with service update complete info")
				}

				return serviceUpdating && dbUpdating && serviceUpdated && dbUpdated
			},
			timeout: 30 * time.Second,
		},
		{
			name: "service stop (windows)",
			run: func(t *testing.T) bool {
				err = dockerClient.ServiceRemove(ctx, targetServiceWin)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					dockerStopped = false
					dbStopped     = false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				stopDocker := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(e events.Message) bool {
						if e.Type != "service" {
							return false
						}
						if e.Action != "remove" {
							return false
						}
						if v, ok := e.Actor.Attributes["name"]; ok {
							if v != "Harness-2080tcp" {
								return false
							}
						} else {
							return false
						}
						return true
					},
				}, dockerMonitor)

				stopDB := helper.TimeoutTester(timeoutCtx, []interface{}{
					timeoutCtx,
					func(d map[string]interface{}) bool {
						var c map[string]interface{}
						if v, ok := d["new_val"]; !ok {
							return false
						} else {
							c = v.(map[string]interface{})
						}
						if c["ServiceName"].(string) != "Harness-2080tcp" {
							return false
						}
						if c["DesiredState"].(string) != "" {
							return false
						}
						if c["State"].(string) != "Stopped" {
							return false
						}
						if c["ServiceID"].(string) == "" {
							return false
						}
						return true
					},
				}, dbMonitor)

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
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
					t.Errorf("service stop not detected in Docker")
				}
				if !dbStopped {
					t.Errorf("db not updated with service stop info")
				}

				return dockerStopped && dbStopped
			},
			timeout: 30 * time.Second,
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

			timeoutCtx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()
			events, _ := dockerClient.Events(timeoutCtx, types.EventsOptions{})
			errs := EventUpdate(events)
			// have to get errors so they don't block
			go func() {
				for e := range errs {
					_ = e
				}
			}()

			time.Sleep(3 * time.Second)
			assert.True(t, tt.run(t))
			assert.True(t, <-res)
			time.Sleep(3 * time.Second)
		})
	}
	test.KillService(ctx, dockerClient, brainID)
	test.DockerCleanUp(ctx, dockerClient, netID)
	os.Setenv("STAGE", oldStage)
}

func Test_updatePluginStatus(t *testing.T) {
	oldStage := os.Getenv("STAGE")
	os.Setenv("STAGE", "TESTING")

	ctx := context.Background()
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

	_, err = r.DB("Controller").Table("Plugins").Insert(map[string]interface{}{
		"Name":          "TestPlugin",
		"ServiceID":     "",
		"ServiceName":   "testing",
		"DesiredState":  "",
		"State":         "Available",
		"Interface":     "192.168.1.1",
		"ExternalPorts": []string{"1080/tcp"},
		"InternalPorts": []string{"1080/tcp"},
		"OS":            string(PluginOSAll),
	}).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	type args struct {
		serviceName string
		update      map[string]string
	}
	tests := []struct {
		name    string
		args    args
		wantDB  map[string]interface{}
		wantErr bool
		err     error
	}{
		{
			name: "service start",
			args: args{
				serviceName: "testing",
				update: map[string]string{
					"State":        "Active",
					"ServiceID":    "hfaldfhak87dfhsddfvns0naef",
					"DesiredState": "",
				},
			},
			wantDB: map[string]interface{}{
				"Name":          "TestPlugin",
				"ServiceID":     "hfaldfhak87dfhsddfvns0naef",
				"ServiceName":   "testing",
				"DesiredState":  "",
				"State":         "Active",
				"Interface":     "192.168.1.1",
				"ExternalPorts": []string{"1080/tcp"},
				"InternalPorts": []string{"1080/tcp"},
				"OS":            string(PluginOSAll),
			},
		},
		{
			name: "service update",
			args: args{
				serviceName: "testing",
				update: map[string]string{
					"State":        "Restarting",
					"DesiredState": "",
				},
			},
			wantDB: map[string]interface{}{
				"Name":          "TestPlugin",
				"ServiceID":     "hfaldfhak87dfhsddfvns0naef",
				"ServiceName":   "testing",
				"DesiredState":  "",
				"State":         "Restarting",
				"Interface":     "192.168.1.1",
				"ExternalPorts": []string{"1080/tcp"},
				"InternalPorts": []string{"1080/tcp"},
				"OS":            string(PluginOSAll),
			},
		},
		{
			name: "service remove",
			args: args{
				serviceName: "testing",
				update: map[string]string{
					"State":        "Stopped",
					"DesiredState": "",
				},
			},
			wantDB: map[string]interface{}{
				"Name":          "TestPlugin",
				"ServiceID":     "hfaldfhak87dfhsddfvns0naef",
				"ServiceName":   "testing",
				"DesiredState":  "",
				"State":         "Stopped",
				"Interface":     "192.168.1.1",
				"ExternalPorts": []string{"1080/tcp"},
				"InternalPorts": []string{"1080/tcp"},
				"OS":            string(PluginOSAll),
			},
		},
		{
			name: "bad service",
			args: args{
				serviceName: "testingbad",
				update: map[string]string{
					"State":        "Active",
					"ServiceID":    "hfaldfhak87dfhsddfvns0naef",
					"DesiredState": "",
				},
			},
			wantErr: true,
			err:     fmt.Errorf("no plugin to update"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc map[string]interface{}
			if err := updatePluginStatus(tt.args.serviceName, tt.args.update); (err != nil) != tt.wantErr {
				t.Errorf("updatePluginStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				assert.Equal(t, tt.err, err)
				return
			}
			cursor, err := r.DB("Controller").Table("Plugins").Run(session)
			if err != nil {
				t.Errorf("rethink error: %v", err)
				return
			}
			if !cursor.Next(&doc) {
				t.Errorf("cursor empty")
				return
			}
			assert.Equal(t, tt.wantDB["Name"].(string), doc["Name"].(string))
			assert.Equal(t, tt.wantDB["ServiceID"].(string), doc["ServiceID"].(string))
			assert.Equal(t, tt.wantDB["ServiceName"].(string), doc["ServiceName"].(string))
			assert.Equal(t, tt.wantDB["DesiredState"].(string), doc["DesiredState"].(string))
			assert.Equal(t, tt.wantDB["State"].(string), doc["State"].(string))
			assert.Equal(t, tt.wantDB["Interface"].(string), doc["Interface"].(string))
			assert.Equal(t, tt.wantDB["ExternalPorts"].([]string)[0], doc["ExternalPorts"].([]interface{})[0].(string))
			assert.Equal(t, tt.wantDB["InternalPorts"].([]string)[0], doc["InternalPorts"].([]interface{})[0].(string))
			assert.Equal(t, tt.wantDB["OS"].(string), doc["OS"].(string))
		})
	}
	test.KillService(ctx, dockerClient, brainID)
	os.Setenv("STAGE", oldStage)
}

func Test_handleContainer(t *testing.T) {
	type args struct {
		event events.Message
	}
	tests := []struct {
		name    string
		args    args
		wantSvc string
		wantUpd map[string]string
		wantErr bool
		err     error
	}{
		{
			name: "container healthy event",
			args: args{
				event: events.Message{
					Status: "health_status: healthy",
					Actor: events.Actor{
						Attributes: map[string]string{
							"com.docker.swarm.service.id":   "testserviceid",
							"com.docker.swarm.service.name": "testservice",
						},
					},
				},
			},
			wantSvc: "testservice",
			wantUpd: map[string]string{
				"State":        "Active",
				"ServiceID":    "testserviceid",
				"DesiredState": "",
			},
		},
		{
			name: "container healthy event 2",
			args: args{
				event: events.Message{
					Action: "health_status: healthy",
					Actor: events.Actor{
						Attributes: map[string]string{
							"com.docker.swarm.service.id":   "testserviceid",
							"com.docker.swarm.service.name": "testservice",
						},
					},
				},
			},
			wantSvc: "testservice",
			wantUpd: map[string]string{
				"State":        "Active",
				"ServiceID":    "testserviceid",
				"DesiredState": "",
			},
		},
		{
			name: "container unhealthy event",
			args: args{
				event: events.Message{
					Status: "health_status: unhealthy",
					Actor: events.Actor{
						Attributes: map[string]string{
							"com.docker.swarm.service.id":   "testserviceid",
							"com.docker.swarm.service.name": "testservice",
						},
					},
				},
			},
			wantSvc: "testservice",
			wantUpd: map[string]string{
				"State":        "Stopped",
				"DesiredState": "",
			},
		},
		{
			name: "container unhealthy event 2",
			args: args{
				event: events.Message{
					Action: "health_status: unhealthy",
					Actor: events.Actor{
						Attributes: map[string]string{
							"com.docker.swarm.service.id":   "testserviceid",
							"com.docker.swarm.service.name": "testservice",
						},
					},
				},
			},
			wantSvc: "testservice",
			wantUpd: map[string]string{
				"State":        "Stopped",
				"DesiredState": "",
			},
		},
		{
			name: "container die event",
			args: args{
				event: events.Message{
					Action: "die",
					Actor: events.Actor{
						Attributes: map[string]string{
							"com.docker.swarm.service.id":   "testserviceid",
							"com.docker.swarm.service.name": "testservice",
						},
					},
				},
			},
			wantSvc: "testservice",
			wantUpd: map[string]string{
				"State":        "Stopped",
				"DesiredState": "",
			},
		},
		{
			name: "empty service name",
			args: args{
				event: events.Message{
					Action: "health_status: unhealthy",
					Actor: events.Actor{
						Attributes: map[string]string{
							"com.docker.swarm.service.id": "testserviceid",
						},
					},
				},
			},
			wantSvc: "",
			wantUpd: map[string]string{},
			wantErr: true,
			err: fmt.Errorf("unhandled container event: %+v", events.Message{
				Action: "health_status: unhealthy",
				Actor: events.Actor{
					Attributes: map[string]string{
						"com.docker.swarm.service.id": "testserviceid",
					},
				},
			}),
		},
		{
			name: "container kill event (dont get)",
			args: args{
				event: events.Message{
					Action: "kill",
					Actor: events.Actor{
						Attributes: map[string]string{
							"com.docker.swarm.service.name": "testservice",
							"com.docker.swarm.service.id":   "testserviceid",
						},
					},
				},
			},
			wantSvc: "",
			wantUpd: map[string]string{},
			wantErr: true,
			err: fmt.Errorf("unhandled container event: %+v", events.Message{
				Action: "kill",
				Actor: events.Actor{
					Attributes: map[string]string{
						"com.docker.swarm.service.name": "testservice",
						"com.docker.swarm.service.id":   "testserviceid",
					},
				},
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSvc, gotUpd, err := handleContainer(tt.args.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleContainer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotSvc != tt.wantSvc {
				t.Errorf("handleContainer() got = %v, want %v", gotSvc, tt.wantSvc)
			}
			if !reflect.DeepEqual(gotUpd, tt.wantUpd) {
				t.Errorf("handleContainer() got1 = %v, want %v", gotUpd, tt.wantUpd)
			}
		})
	}
}
