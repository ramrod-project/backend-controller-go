package dockerservicemanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/rethink"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
)

func Test_getRethinkHost(t *testing.T) {
	oldEnv := os.Getenv("STAGE")
	tests := []struct {
		name string
		env  string
		want string
	}{
		{
			name: "Dev",
			env:  "DEV",
			want: "rethinkdb",
		},
		{
			name: "Prod",
			env:  "PROD",
			want: "rethinkdb",
		},
		{
			name: "Testing",
			env:  "TESTING",
			want: "127.0.0.1",
		},
		{
			name: "Nil",
			env:  "",
			want: "rethinkdb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("STAGE", tt.env)
			if got := getRethinkHost(); got != tt.want {
				t.Errorf("getRethinkHost() = %v, want %v", got, tt.want)
			}
		})
	}
	os.Setenv("STAGE", oldEnv)
}

func Test_getNodes(t *testing.T) {

	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "Test getting the node and assigning",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getNodes()
			if (err != nil) != tt.wantErr {
				t.Errorf("getNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Len(t, got, 1)
			assert.IsType(t, "string", got[0]["Interface"])
			assert.NotEqual(t, "", got[0]["Interface"])
			assert.IsType(t, "string", got[0]["OS"])
			assert.NotEqual(t, "", got[0]["OS"])
			assert.IsType(t, "string", got[0]["NodeHostName"])
			assert.NotEqual(t, "", got[0]["NodeHostName"])
			assert.IsType(t, []string{}, got[0]["TCPPorts"])
			assert.Equal(t, []string{}, got[0]["TCPPorts"])
			assert.IsType(t, []string{}, got[0]["UDPPorts"])
			assert.Equal(t, []string{}, got[0]["UDPPorts"])
			res, err := dockerClient.NodeList(ctx, types.NodeListOptions{})
			if err != nil {
				t.Errorf("%v", err)
				return
			}
			assert.Equal(t, "posix", res[0].Spec.Annotations.Labels["os"])
			assert.Equal(t, got[0]["Interface"], res[0].Spec.Annotations.Labels["ip"])
		})
	}
}

func Test_advertiseIPs(t *testing.T) {
	oldEnv := os.Getenv("STAGE")
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

	session, brainID, err := test.StartBrain(ctx, t, dockerClient, test.BrainSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	type args struct {
		entries []map[string]interface{}
	}
	tests := []struct {
		name     string
		existing map[string]interface{}
		args     args
		wantErr  bool
		err      error
	}{
		{
			name: "One node",
			args: args{
				entries: []map[string]interface{}{
					map[string]interface{}{
						"Interface":    "192.168.1.1",
						"NodeHostName": "ubuntu",
						"OS":           "posix",
						"TCPPorts":     []string{},
						"UDPPorts":     []string{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Two node",
			args: args{
				entries: []map[string]interface{}{
					map[string]interface{}{
						"Interface":    "192.168.1.1",
						"NodeHostName": "ubuntu",
						"OS":           "posix",
						"TCPPorts":     []string{},
						"UDPPorts":     []string{},
					},
					map[string]interface{}{
						"Interface":    "192.168.1.2",
						"NodeHostName": "WIN1935U21",
						"OS":           "nt",
						"TCPPorts":     []string{},
						"UDPPorts":     []string{},
					},
				},
			},
		},
		{
			name: "Existing node",
			existing: map[string]interface{}{
				"Interface":    "10.0.0.1",
				"NodeHostName": "ubuntu",
				"OS":           "posix",
				"TCPPorts":     []string{},
				"UDPPorts":     []string{},
			},
			args: args{
				entries: []map[string]interface{}{
					map[string]interface{}{
						"Interface":    "10.0.0.3",
						"NodeHostName": "ubuntu",
						"OS":           "posix",
						"TCPPorts":     []string{},
						"UDPPorts":     []string{},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "No node",
			args: args{
				entries: []map[string]interface{}{},
			},
			wantErr: true,
			err:     errors.New("no nodes to advertise"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.existing != nil {
				_, err := r.DB("Controller").Table("Ports").Insert(tt.existing).RunWrite(session)
				if err != nil {
					t.Errorf("Rethink error: %v", err)
					return
				}
				time.Sleep(time.Second)
			}
			if err := advertiseIPs(tt.args.entries); (err != nil) != tt.wantErr {
				t.Errorf("advertiseIPs() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				assert.Equal(t, tt.err, err)
				return
			}
			cursor, err := r.DB("Controller").Table("Ports").Run(session)
			if err != nil {
				t.Errorf("Rethink error: %v", err)
				return
			}
			var doc map[string]interface{}
			i := 0
			for cursor.Next(&doc) {
				index := 0
				if doc["OS"] == "nt" {
					index = 1
				}
				assert.Equal(t, tt.args.entries[index]["Interface"], doc["Interface"])
				assert.Equal(t, tt.args.entries[index]["NodeHostName"], doc["NodeHostName"])
				assert.Equal(t, tt.args.entries[index]["OS"], doc["OS"])
				assert.Equal(t, len(tt.args.entries[index]["TCPPorts"].([]string)), len(doc["TCPPorts"].([]interface{})))
				assert.Equal(t, len(tt.args.entries[index]["UDPPorts"].([]string)), len(doc["UDPPorts"].([]interface{})))
				for j, v := range doc["TCPPorts"].([]interface{}) {
					assert.Equal(t, tt.args.entries[index]["TCPPorts"].([]string)[j], v.(string))
				}
				for j, v := range doc["UDPPorts"].([]interface{}) {
					assert.Equal(t, tt.args.entries[index]["UDPPorts"].([]string)[j], v.(string))
				}
				i++
			}
			if i == 0 {
				t.Error("empty cursor result")
				return
			}
			_, err = r.DB("Controller").Table("Ports").Delete().Run(session)
			if err != nil {
				t.Errorf("%v", err)
				return
			}
		})
	}
	test.KillService(ctx, dockerClient, brainID)
	os.Setenv("STAGE", oldEnv)

	test.DockerCleanUp(ctx, dockerClient, "")
}

func Test_getPlugins(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    []ManifestPlugin
		wantErr bool
		err     error
	}{
		{
			name: "Basic",
			content: []byte(`
				[{"Name":"Harness", "OS":"all"}]
			`),
			want: []ManifestPlugin{
				ManifestPlugin{
					Name: "Harness",
					OS:   rethink.PluginOSAll,
				},
			},
			wantErr: false,
		},
		{
			name: "Advanced",
			content: []byte(`
				[{"Name":"Harness", "OS":"all"},{"Name":"OtherPlugin", "OS":"nt"},{"Name":"OtherPlugin2", "OS":"posix"}]
			`),
			want: []ManifestPlugin{
				ManifestPlugin{
					Name: "Harness",
					OS:   rethink.PluginOSAll,
				},
				ManifestPlugin{
					Name: "OtherPlugin",
					OS:   rethink.PluginOSWindows,
				},
				ManifestPlugin{
					Name: "OtherPlugin2",
					OS:   rethink.PluginOSPosix,
				},
			},
			wantErr: false,
		},
		{
			name: "Empty",
			content: []byte(`
				[]
			`),
			want:    []ManifestPlugin{},
			wantErr: true,
			err:     errors.New("no plugins found in manifest.json"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugins := []ManifestPlugin{}
			err := json.Unmarshal(tt.content, &plugins)
			if err != nil {
				t.Errorf("%v", err)
				return
			}
			pluginJSON, _ := json.Marshal(plugins)
			err = ioutil.WriteFile("manifest.json", pluginJSON, 0644)
			got, err := getPlugins()
			if (err != nil) != tt.wantErr {
				t.Errorf("getPlugins() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				assert.Equal(t, tt.err, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPlugins() = %v, want %v", got, tt.want)
			}
			os.Remove("manifest.json")
		})
	}
}

func Test_advertisePlugins(t *testing.T) {
	oldEnv := os.Getenv("STAGE")
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

	session, brainID, err := test.StartBrain(ctx, t, dockerClient, test.BrainSpec)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	tests := []struct {
		name     string
		existing map[string]interface{}
		manifest []ManifestPlugin
		want     []map[string]interface{}
		wantErr  bool
		err      error
	}{
		{
			name: "One plugin",
			manifest: []ManifestPlugin{
				ManifestPlugin{
					Name:  "TestPlugin",
					OS:    rethink.PluginOSAll,
					Extra: true,
				},
			},
			want: []map[string]interface{}{
				map[string]interface{}{
					"Name":          "TestPlugin",
					"ServiceID":     "",
					"ServiceName":   "",
					"DesiredState":  "",
					"State":         "Available",
					"Interface":     "",
					"ExternalPorts": []string{},
					"InternalPorts": []string{},
					"OS":            string(rethink.PluginOSAll),
					"Environment":   []string{},
					"Extra":         true,
				},
			},
			wantErr: false,
		},
		{
			name: "Two plugin",
			manifest: []ManifestPlugin{
				ManifestPlugin{
					Name: "TestPlugin2",
					OS:   rethink.PluginOSPosix,
				},
				ManifestPlugin{
					Name:  "TestPlugin3",
					OS:    rethink.PluginOSWindows,
					Extra: false,
				},
			},
			want: []map[string]interface{}{
				map[string]interface{}{
					"Name":          "TestPlugin2",
					"ServiceID":     "",
					"ServiceName":   "",
					"DesiredState":  "",
					"State":         "Available",
					"Interface":     "",
					"ExternalPorts": []string{},
					"InternalPorts": []string{},
					"OS":            string(rethink.PluginOSPosix),
					"Environment":   []string{},
					"Extra":         false,
				},
				map[string]interface{}{
					"Name":          "TestPlugin3",
					"ServiceID":     "",
					"ServiceName":   "",
					"DesiredState":  "",
					"State":         "Available",
					"Interface":     "",
					"ExternalPorts": []string{},
					"InternalPorts": []string{},
					"OS":            string(rethink.PluginOSWindows),
					"Environment":   []string{},
					"Extra":         false,
				},
			},
			wantErr: false,
		},
		{
			name: "Existing plugin",
			existing: map[string]interface{}{
				"Name":          "ExistingPlugin",
				"ServiceID":     "",
				"ServiceName":   "",
				"DesiredState":  "",
				"State":         "Available",
				"Interface":     "",
				"ExternalPorts": []string{},
				"InternalPorts": []string{},
				"OS":            string(rethink.PluginOSAll),
				"Environment":   []string{},
				"Extra":         true,
			},
			manifest: []ManifestPlugin{
				ManifestPlugin{
					Name:  "ExistingPlugin",
					OS:    rethink.PluginOSPosix,
					Extra: true,
				},
			},
			want: []map[string]interface{}{
				map[string]interface{}{
					"Name":          "ExistingPlugin",
					"ServiceID":     "",
					"ServiceName":   "",
					"DesiredState":  "",
					"State":         "Available",
					"Interface":     "",
					"ExternalPorts": []string{},
					"InternalPorts": []string{},
					"OS":            string(rethink.PluginOSAll),
					"Environment":   []string{},
					"Extra":         true,
				},
			},
			wantErr: false,
		},
		{
			name:     "No plugin",
			manifest: []ManifestPlugin{},
			wantErr:  true,
			err:      errors.New("no plugins to advertise"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.existing != nil {
				_, err := r.DB("Controller").Table("Plugins").Insert(tt.existing).RunWrite(session)
				if err != nil {
					t.Errorf("%v", err)
					return
				}
			}
			err := advertisePlugins(tt.manifest)
			if (err != nil) != tt.wantErr {
				t.Errorf("%v", err)
				return
			} else if tt.wantErr {
				assert.Equal(t, tt.err, err)
				return
			}
			time.Sleep(time.Second)

			cursor, err := r.DB("Controller").Table("Plugins").Run(session)

			var (
				doc   map[string]interface{}
				count int
			)
			for cursor.Next(&doc) {
				index := 0
				found := false
				for i, v := range tt.want {
					if v["Name"].(string) == doc["Name"].(string) {
						found = true
						index = i
						count++
						break
					}
				}
				if !found {
					t.Errorf("Plugin %v not in wanted", doc["Name"])
					continue
				}

				assert.Equal(t, tt.want[index]["Name"], doc["Name"].(string))
				assert.Equal(t, tt.want[index]["ServiceID"], doc["ServiceID"].(string))
				assert.Equal(t, tt.want[index]["ServiceName"], doc["ServiceName"].(string))
				assert.Equal(t, tt.want[index]["DesiredState"], doc["DesiredState"].(string))
				assert.Equal(t, tt.want[index]["State"], doc["State"].(string))
				assert.Equal(t, tt.want[index]["Interface"], doc["Interface"].(string))
				assert.Equal(t, tt.want[index]["OS"], doc["OS"].(string))
				for j, v := range doc["ExternalPorts"].([]interface{}) {
					assert.Equal(t, tt.want[index]["ExternalPorts"].([]string)[j], v.(string))
				}
				for j, v := range doc["InternalPorts"].([]interface{}) {
					assert.Equal(t, tt.want[index]["InternalPorts"].([]string)[j], v.(string))
				}
				for j, v := range doc["Environment"].([]interface{}) {
					assert.Equal(t, tt.want[index]["Environment"].([]string)[j], v.(string))
				}
			}
			assert.Equal(t, len(tt.want), count)
			r.DB("Controller").Table("Plugins").Delete().Run(session)
			time.Sleep(time.Second)
		})
	}

	test.KillService(ctx, dockerClient, brainID)
	os.Setenv("STAGE", oldEnv)

	test.DockerCleanUp(ctx, dockerClient, "")
}

func Test_advertiseStartupService(t *testing.T) {
	oldEnv := os.Getenv("STAGE")
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
	time.Sleep(3 * time.Second)

	tests := []struct {
		name    string
		service map[string]interface{}
		wantP   map[string]interface{}
		wantErr bool
	}{
		{
			name: "harness test",
			service: map[string]interface{}{
				"Name":          "Harness",
				"ServiceID":     "some-random-id",
				"ServiceName":   harnessConfig.ServiceName,
				"DesiredState":  "",
				"State":         "Active",
				"Interface":     "",
				"ExternalPorts": []string{"5000/tcp"},
				"InternalPorts": []string{"5000/tcp"},
				"OS":            string(rethink.PluginOSAll),
				"Environment":   []string{},
			},
			wantP: map[string]interface{}{
				"TCPPorts": []string{"5000"},
				"UDPPorts": []string{},
			},
		},
		{
			name: "aux test",
			service: map[string]interface{}{
				"Name":          "AuxServices",
				"ServiceID":     "some-other-random-id",
				"ServiceName":   auxConfig.ServiceName,
				"DesiredState":  "",
				"State":         "Active",
				"Interface":     "",
				"ExternalPorts": []string{"20/tcp", "21/tcp", "80/tcp", "53/udp"},
				"InternalPorts": []string{"20/tcp", "21/tcp", "80/tcp", "53/udp"},
				"OS":            string(rethink.PluginOSPosix),
				"Environment":   []string{},
			},
			wantP: map[string]interface{}{
				"TCPPorts": []string{"5000", "20", "21", "80"},
				"UDPPorts": []string{"53"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := advertiseStartupService(tt.service); (err != nil) != tt.wantErr {
				t.Errorf("advertiseService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			res, err := r.DB("Controller").Table("Plugins").Filter(map[string]string{"ServiceName": tt.service["ServiceName"].(string)}).Run(session)
			if err != nil {
				t.Errorf("%v", err)
			}
			var (
				doc    map[string]interface{}
				newExt []string
				newInt []string
				newEnv []string
				newTCP []string
				newUDP []string
			)
			assert.True(t, res.Next(&doc))
			assert.Equal(t, tt.service["Name"], doc["Name"].(string))
			assert.Equal(t, tt.service["ServiceID"], doc["ServiceID"].(string))
			assert.Equal(t, tt.service["ServiceName"], doc["ServiceName"].(string))
			assert.Equal(t, tt.service["DesiredState"], doc["DesiredState"].(string))
			assert.Equal(t, tt.service["State"], doc["State"].(string))
			assert.Equal(t, tt.service["Interface"], doc["Interface"].(string))
			assert.Equal(t, tt.service["OS"], doc["OS"].(string))
			for _, v := range doc["ExternalPorts"].([]interface{}) {
				newExt = append(newExt, v.(string))
			}
			for _, v := range doc["InternalPorts"].([]interface{}) {
				newInt = append(newInt, v.(string))
			}
			for _, v := range doc["Environment"].([]interface{}) {
				newEnv = append(newEnv, v.(string))
			}
			for _, v := range tt.service["ExternalPorts"].([]string) {
				assert.Contains(t, newExt, v)
			}
			for _, v := range tt.service["InternalPorts"].([]string) {
				assert.Contains(t, newInt, v)
			}
			for _, v := range tt.service["Environment"].([]string) {
				assert.Contains(t, newEnv, v)
			}

			res, err = r.DB("Controller").Table("Ports").Filter(map[string]string{"NodeHostName": leader}).Run(session)
			if err != nil {
				t.Errorf("%v", err)
				return
			}
			assert.True(t, res.Next(&doc))
			assert.Equal(t, "", doc["Interface"].(string))
			assert.Equal(t, "posix", doc["OS"].(string))
			for _, v := range doc["TCPPorts"].([]interface{}) {
				newTCP = append(newTCP, v.(string))
			}
			for _, v := range doc["UDPPorts"].([]interface{}) {
				newUDP = append(newUDP, v.(string))
			}
			for _, v := range tt.wantP["TCPPorts"].([]string) {
				assert.Contains(t, newTCP, v)
			}
			for _, v := range tt.wantP["UDPPorts"].([]string) {
				assert.Contains(t, newUDP, v)
			}
		})
	}

	test.KillService(ctx, dockerClient, brainID)
	os.Setenv("STAGE", oldEnv)

	//Docker cleanup
	if err := test.DockerCleanUp(ctx, dockerClient, ""); err != nil {
		t.Errorf("cleanup error: %v", err)
	}
}

func Test_getLeaderHostname(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{
			name:    "hostname",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getLeaderHostname()
			if (err != nil) != tt.wantErr {
				t.Errorf("getLeaderHostname() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			fmt.Printf("hostname: %v", got)
		})
	}
}
