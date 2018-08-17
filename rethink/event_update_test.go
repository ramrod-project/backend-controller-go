package rethink

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"

	events "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	r "gopkg.in/gorethink/gorethink.v4"
)

func Test_handleEvent(t *testing.T) {
	ctx := context.TODO()
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

	// a reuseable entry for the plugin table
	testPlugin := map[string]interface{}{
		"Name":          "TestPlugin",
		"ServiceID":     "",
		"ServiceName":   "testing",
		"DesiredState":  "",
		"State":         "Available",
		"Interface":     "192.168.1.1",
		"ExternalPorts": []string{"1080/tcp"},
		"InternalPorts": []string{"1080/tcp"},
		"OS":            string(PluginOSAll),
	}

	// insert a service to update
	_, err = r.DB("Controller").Table("Plugins").Insert(testPlugin).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	time.Sleep(5 * time.Second)

	type args struct {
		event   events.Message
		session *r.Session
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
		err     error
	}{
		// TODO: Add test cases.
		{
			name: "Test response to creating service",
			args: args{
				event: events.Message{
					Type:   "service",
					Action: "create",
					Actor: events.Actor{
						ID: "hfaldfhak87dfhsddfvns0naef",
						Attributes: map[string]string{
							"name": "testing",
						},
					},
				},
				session: session,
			},
			wantErr: false,
			want: map[string]interface{}{
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
			name: "Test response to removing service",
			args: args{
				event: events.Message{
					Type:   "service",
					Action: "remove",
					Actor: events.Actor{
						ID: "hfaldfhak87dfhsddfvns0naef",
						Attributes: map[string]string{
							"name": "testing",
						},
					},
				},
				session: session,
			},
			wantErr: false,
			want: map[string]interface{}{
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
			name: "Test response to updating service",
			args: args{
				event: events.Message{
					Type: "service",
					Actor: events.Actor{
						ID: "hfaldfhak87dfhsddfvns0naef",
						Attributes: map[string]string{
							"name":            "testing",
							"updatestate.new": "updating",
						},
					},
				},
				session: session,
			},
			wantErr: false,
			want: map[string]interface{}{
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
			name: "Test response to completing service",
			args: args{
				event: events.Message{
					Type: "service",
					Actor: events.Actor{
						ID: "hfaldfhak87dfhsddfvns0naef",
						Attributes: map[string]string{
							"name":            "testing",
							"updatestate.new": "completed",
						},
					},
				},
				session: session,
			},
			wantErr: false,
			want: map[string]interface{}{
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
			name: "Test no name",
			args: args{
				event: events.Message{
					Type:   "service",
					Action: "create",
					Actor: events.Actor{
						ID: "hfaldfhak87dfhsddfvns0naef",
					},
				},
				session: session,
			},
			wantErr: true,
			want: map[string]interface{}{
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
			err: fmt.Errorf("no Name Attribute"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handleEvent(tt.args.event, tt.args.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				assert.Equal(t, tt.err, err)
			}
			filter := map[string]string{
				"ServiceName": "testing",
			}
			res, err := r.DB("Controller").Table("Plugins").Filter(filter).Run(session)
			if err != nil {
				t.Errorf("handleEvent fail: %v", err)
				return
			}
			var doc map[string]interface{}
			if ok := res.Next(&doc); !ok {
				t.Errorf("handleEvent: Empty Cursor")
				return
			}
			if _, ok := doc["State"]; ok {
				assert.Equal(t, tt.want["State"], doc["State"])
			}
			if _, ok := doc["State"]; ok {
				assert.Equal(t, tt.want["DesiredState"], doc["DesiredState"])
			}
		})
	}

	test.KillService(ctx, dockerClient, brainID)
}

func TestEventUpdate(t *testing.T) {
	type args struct {
		in <-chan events.Message
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
			got := EventUpdate(tt.args.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EventUpdate() got = %v, want %v", got, tt.want)
			}
		})
	}
}
