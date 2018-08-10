package rethink

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/docker/docker/api/types"
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

	// Start brain
	result, err := dockerClient.ServiceCreate(ctx, brainSpec, types.ServiceCreateOptions{})
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// create a rethink connection
	session, err := r.Connect(r.ConnectOpts{
		Address: "127.0.0.1",
	})
	start := time.Now()
	if err != nil {
		for {
			if time.Since(start) >= 20*time.Second {
				t.Errorf("%v", err)
				return
			}
			session, err = r.Connect(r.ConnectOpts{
				Address: "127.0.0.1",
			})
			if err == nil {
				_, err := r.DB("Controller").Table("Plugins").Run(session)
				if err == nil {
					break
				}
			}
			time.Sleep(time.Second)
		}
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handleEvent(tt.args.event, tt.args.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
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
			assert.Equal(t, tt.want["State"], doc["State"])
			assert.Equal(t, tt.want["DesiredState"], doc["DesiredState"])
		})
	}

	dockerClient.ServiceRemove(ctx, result.ID)
}

func TestEventUpdate(t *testing.T) {
	type args struct {
		in <-chan events.Message
	}
	tests := []struct {
		name  string
		args  args
		want  <-chan r.WriteResponse
		want1 <-chan error
	}{
		// TODO: Add test cases.
		// {
		// 	name: "Eventhandle test",
		// 	args: args{
		// 		event: events.Message{
		// 			Type:   "service",
		// 			Action: "remove",
		// 			Actor: events.Actor{
		// 				ID: "hfaldfhak87dfhsddfvns0naef",
		// 				Attributes: map[string]string{
		// 					"name": "testing",
		// 				},
		// 			},
		// 		},
		// 	},
		// 	want: gorethink.WriteResponse{
		// 		Replaced: 1,
		// 	},
		// 	want1: nil,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := EventUpdate(tt.args.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EventUpdate() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("EventUpdate() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
