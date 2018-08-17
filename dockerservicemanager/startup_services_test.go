package dockerservicemanager

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/helper"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
)

func TestStartupServices(t *testing.T) {
	oldStage := os.Getenv("STAGE")
	os.Setenv("STAGE", "TESTING")
	oldHarness := os.Getenv("START_HARNESS")
	os.Setenv("START_HARNESS", "YES")
	oldAux := os.Getenv("START_AUX")
	os.Setenv("START_AUX", "YES")

	ctx := context.Background()
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

	// Start the brain
	session, brainID, err := test.StartBrain(ctx, t, dockerClient, intBrainSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	leader, err := getLeaderHostname()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	_, err = r.DB("Controller").Table("Ports").Insert(
		map[string]interface{}{
			"Interface":    "",
			"NodeHostName": leader,
			"OS":           "posix",
			"TCPPorts":     []string{},
			"UDPPorts":     []string{},
		}).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	time.Sleep(3 * time.Second)

	tests := []struct {
		name    string
		wait    func(*testing.T, time.Duration) bool
		wantErr bool
		timeout time.Duration
	}{
		{
			name:    "startup",
			wantErr: false,
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					harnessStarted, auxStarted, portUpdated = false, false, false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				startAux := helper.TimeoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							if e.Type != "container" {
								break
							}
							if e.Action != "start" {
								break
							}
							if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
								if v != "AuxiliaryServices" {
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

				startHarness := helper.TimeoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							if e.Type != "container" {
								break
							}
							if e.Action != "start" {
								break
							}
							if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
								if v != "Harness-5000" {
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

				startDB := helper.TimeoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
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
							for {
								select {
								case <-timeoutCtx.Done():
									return
								default:
									break
								}
								cursor, err := r.DB("Controller").Table("Ports").Run(s)
								if err != nil {
									log.Println(fmt.Errorf("%v", err))
									errs <- err
								}
								var doc map[string]interface{}
								if cursor.Next(&doc) {
									changes <- doc
								}
								time.Sleep(500 * time.Millisecond)
							}
						}()
						return changes, errs
					}(sessionTest)

					for {
					S:
						select {
						case <-cntxt.Done():
							return false
						case e := <-errChan:
							log.Println(fmt.Errorf("%v", e))
							return false
						case d := <-changeChan:
							log.Printf("change: %+v", d)
							if _, ok := d["Interface"]; ok {
								if d["Interface"].(string) != "" {
									break
								}
							}
							for _, p := range []string{"20", "21", "80", "5000"} {
								found := false
								for _, pd := range d["TCPPorts"].([]interface{}) {
									if pd.(string) == p {
										found = true
									}
								}
								if !found {
									break S
								}
							}
							for _, p := range []string{"53"} {
								found := true
								for _, pd := range d["UDPPorts"].([]interface{}) {
									if pd.(string) == p {
										found = true
									}
								}
								if !found {
									break S
								}
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
						<-startHarness
						<-startAux
						log.Printf("Done (main)")
						break L
					case v := <-startHarness:
						if v {
							log.Printf("Setting harnessStarted to %v", v)
							harnessStarted = v
						}
					case v := <-startAux:
						if v {
							log.Printf("Setting auxStarted to %v", v)
							auxStarted = v
						}
					case v := <-startDB:
						if v {
							log.Printf("Setting auxStarted to %v", v)
							portUpdated = v
						}
					default:
						break
					}
					if harnessStarted && auxStarted && portUpdated {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !harnessStarted {
					t.Errorf("Harness start event not detected")
				}
				if !auxStarted {
					t.Errorf("Aux start event not detected")
				}
				if !portUpdated {
					t.Errorf("Port entry not updated")
				}

				return harnessStarted && auxStarted && portUpdated

			},
			timeout: 30 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := make(chan bool)
			start := make(chan bool)
			timeoutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			go func() {
				res <- tt.wait(t, tt.timeout)
				close(res)
				return
			}()
			time.Sleep(3 * time.Second)
			go func() {
				err := StartupServices()
				if (err != nil) != tt.wantErr {
					t.Errorf("StartupServices() error = %v, wantErr %v", err, tt.wantErr)
				}
				start <- true
			}()

			select {
			case <-timeoutCtx.Done():
				t.Errorf("StartupServices() still running")
			case tr := <-start:
				assert.True(t, tr)
			}
			assert.True(t, <-res)
		})
	}

	test.KillService(ctx, dockerClient, brainID)
	test.DockerCleanUp(ctx, dockerClient, netRes.ID)
	os.Setenv("START_HARNESS", oldHarness)
	os.Setenv("START_AUX", oldAux)
	os.Setenv("STAGE", oldStage)
}