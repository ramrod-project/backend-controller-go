package rethink

import (
	"context"
	"log"
	"os"
	"reflect"
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
		log     customtypes.ContainerLog
		wantErr bool
	}{
		{
			name: "test log 1",
			log: customtypes.ContainerLog{
				ContainerID:   "284813vm8y13-13v8y9-713yv1",
				ContainerName: "some-service-name.0whatever",
				Log:           "[INFO] blahblahblahblhbq 39 4g0wo 43589pqhwpr8g4",
				ServiceName:   "some-service-name",
				LogTimestamp:  int32(time.Now().Unix()),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.SetTags("rethinkdb", "json")

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
							log.Printf("%+v", doc)
							if v, ok := doc["containerID"]; ok {
								assert.Equal(t, tt.log.ContainerID, v.(string))
							} else {
								continue L
							}
							if v, ok := doc["containerName"]; ok {
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
								now := int32(time.Now().Unix())
								assert.True(t, (int32(v.(float64)) >= now-10))
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
}

func TestAggregateLogs(t *testing.T) {
	type args struct {
		ctx      context.Context
		logChans <-chan (<-chan customtypes.ContainerLog)
	}
	tests := []struct {
		name string
		args args
		want <-chan error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AggregateLogs(tt.args.ctx, tt.args.logChans); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AggregateLogs() = %v, want %v", got, tt.want)
			}
		})
	}
}
