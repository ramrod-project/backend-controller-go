package dockerservicemanager

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	events "github.com/docker/docker/api/types/events"
	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/helper"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
)

func TestEventMonitor(t *testing.T) {

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

	var targetService string

	tests := []struct {
		name    string
		run     func(t *testing.T) bool
		wait    func(t *testing.T, evts <-chan events.Message, timeout time.Duration) bool
		timeout time.Duration
	}{
		{
			name: "container healthy",
			run: func(t *testing.T) bool {
				testSvcHealthCheck := testServiceSpec
				testSvcHealthCheck.TaskTemplate.ContainerSpec.Image = "ramrodpcp/interpreter-plugin:" + getTagFromEnv()
				testSvcHealthCheck.TaskTemplate.ContainerSpec.Command = []string{"sleep", "300"}
				testSvcHealthCheck.TaskTemplate.ContainerSpec.Healthcheck = &container.HealthConfig{
					Test:     []string{"CMD", "echo", "\"test\""},
					Interval: 2 * time.Second,
					Retries:  3,
				}
				testSvc, err := dockerClient.ServiceCreate(ctx, testSvcHealthCheck, types.ServiceCreateOptions{})
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				targetService = testSvc.ID
				return true
			},
			wait: func(t *testing.T, evts <-chan events.Message, timeout time.Duration) bool {
				var (
					startedEvent = false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				startEvent := helper.TimeoutTester(timeoutCtx, []interface{}{timeoutCtx, evts}, func(args ...interface{}) bool {
					cntxt := args[0].(context.Context)
					evts := args[1].(<-chan events.Message)

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-evts:
							if e.Type != "container" {
								break
							}
							if e.Action != "health_status: healthy" && e.Status != "health_status: healthy" {
								break
							}
							if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
								if v != "TestService" {
									break
								}
							} else {
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

			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-startEvent
						break L
					case v := <-startEvent:
						if v {
							log.Printf("Setting startedEvent to %v", v)
							startedEvent = v
						}
					default:
						break
					}
					if startedEvent {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !startedEvent {
					t.Errorf("start event not received")
				}

				return startedEvent
			},
			timeout: 20 * time.Second,
		},
		{
			name: "service update",
			run: func(t *testing.T) bool {
				testSvcUpdate := testServiceSpec
				testSvcUpdate.TaskTemplate.ContainerSpec.Image = "ramrodpcp/interpreter-plugin:" + getTagFromEnv()
				testSvcUpdate.TaskTemplate.ContainerSpec.Command = []string{"sleep", "300"}
				testSvcUpdate.TaskTemplate.ContainerSpec.Env = []string{"TEST=TEST"}
				testSvcUpdate.TaskTemplate.ContainerSpec.Healthcheck = &container.HealthConfig{
					Test:     []string{"CMD", "echo", "\"test\""},
					Interval: 2 * time.Second,
					Retries:  3,
				}
				insp, _, err := dockerClient.ServiceInspectWithRaw(ctx, targetService)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				_, err = dockerClient.ServiceUpdate(ctx, targetService, insp.Meta.Version, testSvcUpdate, types.ServiceUpdateOptions{})
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, evts <-chan events.Message, timeout time.Duration) bool {
				var (
					updatingEvent = false
					updatedEvent  = false
					eventUpdated  = make(<-chan bool)
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				eventUpdating := helper.TimeoutTester(timeoutCtx, []interface{}{timeoutCtx, evts}, func(args ...interface{}) bool {
					cntxt := args[0].(context.Context)
					evts := args[1].(<-chan events.Message)

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-evts:
							if e.Type != "service" {
								break
							}
							if e.Action != "update" {
								break
							}
							if v, ok := e.Actor.Attributes["name"]; ok {
								if v != "TestService" {
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
						default:
							break
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				defer cancel()

			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-eventUpdating
						break L
					case v := <-eventUpdating:
						if v {
							log.Printf("Setting updatingEvent to %v", v)
							updatingEvent = v
							eventUpdated = helper.TimeoutTester(timeoutCtx, []interface{}{timeoutCtx, evts}, func(args ...interface{}) bool {
								cntxt := args[0].(context.Context)
								evts := args[1].(<-chan events.Message)

								for {
									select {
									case <-cntxt.Done():
										return false
									case e := <-evts:
										if e.Type != "container" {
											break
										}
										if e.Action != "health_status: healthy" && e.Status != "health_status: healthy" {
											break
										}
										if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
											if v != "TestService" {
												break
											}
										} else {
											break
										}
										return true
									default:
										break
									}
									time.Sleep(100 * time.Millisecond)
								}
							})
						}
					case v := <-eventUpdated:
						if v {
							log.Printf("Setting updatedEvent to %v", v)
							updatedEvent = v
						}
					default:
						break
					}
					if updatedEvent && updatingEvent {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !updatingEvent {
					t.Errorf("updating event not received")
				}
				if !updatedEvent {
					t.Errorf("updated event not received")
				}

				return updatedEvent && updatingEvent
			},
			timeout: 30 * time.Second,
		},
		{
			name: "container unhealthy",
			run: func(t *testing.T) bool {
				testSvcHealthCheck := testServiceSpec
				testSvcHealthCheck.TaskTemplate.ContainerSpec.Image = "ramrodpcp/interpreter-plugin:" + getTagFromEnv()
				testSvcHealthCheck.TaskTemplate.ContainerSpec.Command = []string{"sleep", "300"}
				testSvcHealthCheck.TaskTemplate.ContainerSpec.Healthcheck = &container.HealthConfig{
					Test:     []string{"CMD", "exit 1"},
					Interval: 1 * time.Second,
					Retries:  1,
				}
				insp, _, err := dockerClient.ServiceInspectWithRaw(ctx, targetService)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				_, err = dockerClient.ServiceUpdate(ctx, targetService, insp.Meta.Version, testSvcHealthCheck, types.ServiceUpdateOptions{})
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, evts <-chan events.Message, timeout time.Duration) bool {
				var (
					unhealthyEvent = false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				eventUnhealthy := helper.TimeoutTester(timeoutCtx, []interface{}{timeoutCtx, evts}, func(args ...interface{}) bool {
					cntxt := args[0].(context.Context)
					evts := args[1].(<-chan events.Message)

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-evts:
							if e.Type != "container" {
								break
							}
							if e.Action != "health_status: unhealthy" && e.Status != "health_status: unhealthy" {
								break
							}
							if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
								if v != "TestService" {
									break
								}
							} else {
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

			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-eventUnhealthy
						break L
					case v := <-eventUnhealthy:
						if v {
							log.Printf("Setting unhealthyEvent to %v", v)
							unhealthyEvent = v
						}
					default:
						break
					}
					if unhealthyEvent {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !unhealthyEvent {
					t.Errorf("unhealthy event not received")
				}

				return unhealthyEvent
			},
			timeout: 30 * time.Second,
		},
		{
			name: "container dead",
			run: func(t *testing.T) bool {
				err = dockerClient.ServiceRemove(ctx, targetService)
				if err != nil {
					t.Errorf("%v", err)
					return false
				}
				return true
			},
			wait: func(t *testing.T, evts <-chan events.Message, timeout time.Duration) bool {
				var (
					deadEvent = false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				eventDead := helper.TimeoutTester(timeoutCtx, []interface{}{timeoutCtx, evts}, func(args ...interface{}) bool {
					cntxt := args[0].(context.Context)
					evts := args[1].(<-chan events.Message)

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-evts:
							if e.Type != "container" {
								break
							}
							if e.Action != "die" {
								break
							}
							if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
								if v != "TestService" {
									break
								}
							} else {
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

			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-eventDead
						break L
					case v := <-eventDead:
						if v {
							log.Printf("Setting deadEvent to %v", v)
							deadEvent = v
						}
					default:
						break
					}
					if deadEvent {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !deadEvent {
					t.Errorf("dead event not received")
				}

				return deadEvent
			},
			timeout: 20 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := make(chan bool)
			evts, errs := EventMonitor()
			go func() {
				res <- tt.wait(t, evts, tt.timeout)
				close(res)
				return
			}()
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
	if err := test.DockerCleanUp(ctx, dockerClient, ""); err != nil {
		t.Errorf("setup error: %v", err)
	}
}
