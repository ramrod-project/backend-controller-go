package rethink

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/customtypes"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
)

func Test_logSend(t *testing.T) {

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = "latest"
	}

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

	tests := []struct {
		name    string
		log     customtypes.Log
		wantErr bool
	}{
		{
			name: "test log 1",
			log: customtypes.Log{
				ContainerID:   "284813vm8y13-13v8y9-713yv1",
				ContainerName: "some-service-name.0whatever",
				Log:           "[INFO] blahblahblahblhbq 39 4g0wo 43589pqhwpr8g4",
				ServiceName:   "some-service-name",
				LogTimestamp: uint64(time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.SetTags("json")

			if err := logSend(session, tt.log); (err != nil) != tt.wantErr {
				t.Errorf("logSend() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			res, errs := func() (<-chan struct{}, <-chan error) {
				ret := make(chan struct{})
				errs := make(chan error)
				go func() {
					defer close(ret)
					defer close(errs)

					doc := make(map[string]interface{})

				L:
					for {
						select {
						case <-timeoutCtx.Done():
							return
						default:
							break
						}
						time.Sleep(1000 * time.Millisecond)

						c, err := r.DB("Brain").Table("Logs").Run(session)
						if err != nil {
							errs <- err
							return
						}
						log.Printf("checking cursor")
						if c.Next(&doc) {
							if v, ok := doc["ContainerID"]; ok {
								assert.Equal(t, tt.log.ContainerID, v.(string))
							} else {
								continue L
							}
							if v, ok := doc["ContainerName"]; ok {
								assert.Equal(t, tt.log.ContainerName, v.(string))
							} else {
								continue L
							}
							if v, ok := doc["sourceServiceName"]; ok {
								assert.Equal(t, tt.log.ServiceName, v.(string))
							} else {
								continue L
							}
							if v, ok := doc["msg"]; ok {
								assert.Equal(t, tt.log.Log, v.(string))
							} else {
								continue L
							}
							if v, ok := doc["rt"]; ok {
								now := uint64(time.Now().UnixNano() / 1000000),
								assert.True(t, (v.(uint64) >= now-10))
							} else {
								continue L
							}
							ret <- struct{}{}
							return
						}
					}
				}()
				return ret, errs
			}()

			select {
			case <-timeoutCtx.Done():
				t.Errorf("timeout context exceeded")
				return
			case e := <-errs:
				t.Errorf("%v", e)
				return
			case <-res:
				return
			}
		})
	}
	test.KillService(ctx, dockerClient, brainID)
	if err := test.DockerCleanUp(ctx, dockerClient, ""); err != nil {
		t.Errorf("cleanup error: %v", err)
		return
	}
}

func TestAggregateLogs(t *testing.T) {

	oldStage := os.Getenv("STAGE")
	os.Setenv("STAGE", "TESTING")

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = "latest"
	}

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

	tests := []struct {
		name    string
		run     func(context.Context, int) <-chan (<-chan customtypes.Log)
		n       int
		timeout time.Duration
		wait    func(context.Context, int) (<-chan struct{}, <-chan error)
	}{
		{
			name: "test",
			run: func(ctx context.Context, number int) <-chan (<-chan customtypes.Log) {
				ret := make(chan (<-chan customtypes.Log))

				go func() {
					i := 0
					for i < number {
						newChan := func() <-chan customtypes.Log {
							c := make(chan customtypes.Log)
							logs := []string{fmt.Sprintf("%vtest1", i), fmt.Sprintf("%vtest2", i), fmt.Sprintf("%vtest3", i)}
							sName := fmt.Sprintf("TestService%v", i)
							cName := sName + ".0.somerandomstring12345"
							cID := "some-random-id"

							go func() {
								defer close(c)
								for _, log := range logs {
									c <- customtypes.Log{
										ContainerID:   cID,
										ContainerName: cName,
										Log:           log,
										ServiceName:   sName,
										LogTimestamp: uint64(time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))),
									}
								}
								select {
								case <-ctx.Done():
									return
								}
							}()
							return c
						}()
						ret <- newChan
						i++
					}
				}()
				return ret
			},
			wait: func(ctx context.Context, number int) (<-chan struct{}, <-chan error) {

				ret := make(chan struct{})
				errs := make(chan error)
				go func() {
					defer close(ret)
					defer close(errs)

					doc := make(map[string]interface{})

					for {
						select {
						case <-ctx.Done():
							return
						default:
							break
						}
						time.Sleep(1000 * time.Millisecond)

						c, err := r.DB("Brain").Table("Logs").Run(session)
						if err != nil {
							errs <- err
							return
						}
						count := 0
						for c.Next(&doc) {
							count++
						}
						if count == number {
							ret <- struct{}{}
							return
						}
					}
				}()
				return ret, errs
			},
			timeout: 10 * time.Second,
			n:       3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeoutCtx, cancel := context.WithTimeout(ctx, tt.timeout)
			defer cancel()

			res, errs := tt.wait(ctx, tt.n*3)

			chans := tt.run(ctx, tt.n)

			logErrs := AggregateLogs(ctx, chans)

			select {
			case <-timeoutCtx.Done():
				t.Errorf("timeout context exceeded")
				return
			case e := <-errs:
				t.Errorf("%v", e)
				return
			case e := <-logErrs:
				t.Errorf("%v", e)
				return
			case <-res:
				return
			}
		})
	}

	test.KillService(ctx, dockerClient, brainID)
	if err := test.DockerCleanUp(ctx, dockerClient, ""); err != nil {
		t.Errorf("cleanup error: %v", err)
		return
	}
	os.Setenv("STAGE", oldStage)
}
