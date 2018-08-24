package rethink

import (
	"reflect"
	"testing"

	events "github.com/docker/docker/api/types/events"
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

func Test_updatePluginStatus(t *testing.T) {
	type args struct {
		serviceName string
		update      map[string]string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := updatePluginStatus(tt.args.serviceName, tt.args.update); (err != nil) != tt.wantErr {
				t.Errorf("updatePluginStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_handleContainer(t *testing.T) {
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := handleContainer(tt.args.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleContainer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("handleContainer() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("handleContainer() got1 = %v, want %v", got1, tt.want1)
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
		// TODO: Add test cases.
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
