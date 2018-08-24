package rethink

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	events "github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
)

/*func Test_handleEvent(t *testing.T) {
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
}*/

func TestEventUpdate(t *testing.T) {
	type args struct {
		in <-chan events.Message
	}
	tests := []struct {
		name string
		args args
		want <-chan error
	}{
		{
			name: "service start",
		},
		{
			name: "service update",
		},
		{
			name: "service remove",
		},
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

	r.DB("Controller").Table("Plugins").Insert(map[string]interface{}{
		"Name":          "TestPlugin",
		"ServiceID":     "",
		"ServiceName":   "testing",
		"DesiredState":  "Activate",
		"State":         "Available",
		"Interface":     "192.168.1.1",
		"ExternalPorts": []string{"1080/tcp"},
		"InternalPorts": []string{"1080/tcp"},
		"OS":            string(PluginOSAll),
	}).RunWrite(session)

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

func Test_handleService(t *testing.T) {
	type args struct {
		event events.Message
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   map[string]string
		wantErr bool
	}{
		{
			name: "service updating event",
		},
		{
			name: "service create event (dont get)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := handleService(tt.args.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("handleService() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("handleService() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
