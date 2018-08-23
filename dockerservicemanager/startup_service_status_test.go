package dockerservicemanager

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/helper"
	"github.com/ramrod-project/backend-controller-go/rethink"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
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

	// Start the brain
	_, _, err = test.StartBrain(ctx, t, dockerClient, test.BrainSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// Start some services
	_, err = dockerClient.ServiceCreate(ctx, swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: "TestService1",
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				DNSConfig: &swarm.DNSConfig{},
				Env:       []string{"PLUGIN=TestPlugin", "TEST=TEST"},
				Healthcheck: &container.HealthConfig{
					Interval: time.Second,
					Timeout:  time.Second * 3,
					Retries:  3,
				},
				Image:   "alpine:3.7",
				Command: []string{"sleep", "30"},
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
					PublishedPort: 5000,
					TargetPort:    5000,
				},
			},
		},
	}, types.ServiceCreateOptions{})

	_, err = dockerClient.ServiceCreate(ctx, swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: "TestService2",
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				DNSConfig: &swarm.DNSConfig{},
				Env:       []string{"PLUGIN=TestPlugin"},
				Healthcheck: &container.HealthConfig{
					Interval: time.Second,
					Timeout:  time.Second * 3,
					Retries:  3,
				},
				Image:   "alpine:3.7",
				Command: []string{"sleep", "30"},
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
					PublishedPort: 6000,
					TargetPort:    6000,
				},
			},
		},
	}, types.ServiceCreateOptions{})

	tests := []struct {
		name string
		wait func(t *testing.T, timeout time.Duration) bool
		// Set timeout for wait
		timeout time.Duration
		wantErr bool
	}{
		{
			name: "test service 1",
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					dbUpdated = true
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				pluginDB := helper.TimeoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							cursor, err := r.DB("Controller").Table("Ports").Run(s)
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
							if d["new_val"].(map[string]interface{})["ServiceName"].(string) != "TestService1" {
								break
							}
							if d["new_val"].(map[string]interface{})["Name"].(string) != "TestPlugin" {
								break
							}
							if d["new_val"].(map[string]interface{})["State"].(string) != "Active" {
								break
							}
							if len(d["new_val"].(map[string]interface{})["Environment"].([]interface{})) != 2 {
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
						<-pluginDB
						break L
					case v := <-pluginDB:
						if v {
							log.Printf("Setting dbUpdated to %v", v)
							dbUpdated = v
						}
					default:
						break
					}
					if dbUpdated {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !dbUpdated {
					t.Errorf("db not updated with test service 1")
				}

				return dbUpdated
			},
			timeout: 20 * time.Second,
		},
		{
			name: "test service 2",
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					dbUpdated = true
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				pluginDB := helper.TimeoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							cursor, err := r.DB("Controller").Table("Ports").Run(s)
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
							if d["new_val"].(map[string]interface{})["ServiceName"].(string) != "TestService1" {
								break
							}
							if d["new_val"].(map[string]interface{})["Name"].(string) != "TestPlugin" {
								break
							}
							if d["new_val"].(map[string]interface{})["State"].(string) != "Active" {
								break
							}
							if len(d["new_val"].(map[string]interface{})["Environment"].([]interface{})) != 1 {
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
						<-pluginDB
						break L
					case v := <-pluginDB:
						if v {
							log.Printf("Setting dbUpdated to %v", v)
							dbUpdated = v
						}
					default:
						break
					}
					if dbUpdated {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !dbUpdated {
					t.Errorf("db not updated with test service 1")
				}

				return dbUpdated
			},
			timeout: 20 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := StartupServiceStatus(); (err != nil) != tt.wantErr {
				t.Errorf("StartupServiceStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	os.Setenv("STAGE", oldStage)
	// Docker cleanup
	if err := test.DockerCleanUp(ctx, dockerClient, netID); err != nil {
		t.Errorf("cleanup error: %v", err)
	}
}
