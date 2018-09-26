package dockerservicemanager

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	types "github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
)

// TODO:
// get leader node IP to use to verify tests

func Test_CreatePluginService(t *testing.T) {
	env := os.Getenv("STAGE")
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

	testPort := map[string]interface{}{
		"Interface":    GetManagerIP(),
		"TCPPorts":     []string{},
		"UDPPorts":     []string{},
		"NodeHostName": "test",
		"OS":           "posix",
	}
	_, err = r.DB("Controller").Table("Ports").Insert(testPort).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	netID, err := test.CheckCreateNet("test_create")
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	type args struct {
		config PluginServiceConfig
	}
	tests := []struct {
		name    string
		args    args
		want    swarm.Service
		wantErr bool
		err     error
	}{
		{
			name: "Test creating a plugin service",
			args: args{
				config: PluginServiceConfig{
					Address: GetManagerIP(),
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=5000",
						"PLUGIN=Harness",
						"PLUGIN_NAME=Harness-5000tcp",
					},
					Network: "test_create",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    5000,
						PublishedPort: 5000,
						PublishMode:   swarm.PortConfigPublishModeHost,
					}},
					ServiceName: "Harness-5000tcp",
				},
			},
			want: swarm.Service{
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name: "Harness-5000tcp",
					},
					TaskTemplate: swarm.TaskSpec{
						ContainerSpec: swarm.ContainerSpec{
							Image: "ramrodpcp/interpreter-plugin:" + getTagFromEnv(),
							Env: []string{
								"STAGE=DEV",
								"LOGLEVEL=DEBUG",
								"PORT=5000",
								"PLUGIN=Harness",
								"PLUGIN_NAME=Harness-5000tcp",
								"RETHINK_HOST=" + GetManagerIP(),
							},
						},
						Networks: []swarm.NetworkAttachmentConfig{
							swarm.NetworkAttachmentConfig{
								Target: netID,
							},
						},
					},
					EndpointSpec: &swarm.EndpointSpec{
						Mode: swarm.ResolutionModeVIP,
						Ports: []swarm.PortConfig{
							swarm.PortConfig{
								Protocol:      swarm.PortConfigProtocolTCP,
								TargetPort:    uint32(5000),
								PublishedPort: uint32(5000),
								PublishMode:   swarm.PortConfigPublishModeHost,
							},
						},
					},
				},
			},
		},
		{
			name: "Bad network",
			args: args{
				config: PluginServiceConfig{
					Address: GetManagerIP(),
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=6000",
						"PLUGIN=Harness",
						"PLUGIN_NAME=Harness-6000tcp",
					},
					Network: "blah",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    6000,
						PublishedPort: 6000,
						PublishMode:   swarm.PortConfigPublishModeHost,
					}},
					ServiceName: "Harness-6000tcp",
				},
			},
			want:    swarm.Service{},
			wantErr: true,
			err:     errors.New("Error response from daemon: network blah not found"),
		},
		{
			name: "Duplicate service name",
			args: args{
				config: PluginServiceConfig{
					Address: GetManagerIP(),
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=5000",
						"PLUGIN=Harness",
						"PLUGIN_NAME=Harness-5000tcp",
					},
					Network: "test_create",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    5001,
						PublishedPort: 5001,
						PublishMode:   swarm.PortConfigPublishModeHost,
					}},
					ServiceName: "Harness-5000tcp",
				},
			},
			want:    swarm.Service{},
			wantErr: true,
			err:     errors.New("Error response from daemon: rpc error: code = Unknown desc = name conflicts with an existing object"),
		},
		{
			name: "Test creating an 'extra' plugin service",
			args: args{
				config: PluginServiceConfig{
					Address: GetManagerIP(),
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=7000",
						"PLUGIN=Harness",
						"PLUGIN_NAME=Harness-7000tcp",
					},
					Extra:   true,
					Network: "test_create",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    7000,
						PublishedPort: 7000,
						PublishMode:   swarm.PortConfigPublishModeHost,
					}},
					ServiceName: "Harness-7000tcp",
				},
			},
			want: swarm.Service{
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name: "Harness-7000tcp",
					},
					TaskTemplate: swarm.TaskSpec{
						ContainerSpec: swarm.ContainerSpec{
							Image: "ramrodpcp/interpreter-plugin-extra:" + getTagFromEnv(),
							Env: []string{
								"STAGE=DEV",
								"LOGLEVEL=DEBUG",
								"PORT=7000",
								"PLUGIN=Harness",
								"PLUGIN_NAME=Harness-7000tcp",
								"RETHINK_HOST=" + GetManagerIP(),
							},
						},
						Networks: []swarm.NetworkAttachmentConfig{
							swarm.NetworkAttachmentConfig{
								Target: netID,
							},
						},
					},
					EndpointSpec: &swarm.EndpointSpec{
						Mode: swarm.ResolutionModeVIP,
						Ports: []swarm.PortConfig{
							swarm.PortConfig{
								Protocol:      swarm.PortConfigProtocolTCP,
								TargetPort:    uint32(7000),
								PublishedPort: uint32(7000),
								PublishMode:   swarm.PortConfigPublishModeHost,
							},
						},
					},
				},
			},
		},
	}
	generatedIDs := make([]string, len(tests))
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreatePluginService(&tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePluginService() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				assert.Equal(t, tt.err, err)
				return
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			res, errs := func(id string) (<-chan swarm.Service, <-chan error) {
				ret := make(chan swarm.Service)
				errs := make(chan error)
				go func() {
					defer close(ret)
					defer close(errs)
					for {
						select {
						case <-timeoutCtx.Done():
							return
						default:
							break
						}
						insp, _, err := dockerClient.ServiceInspectWithRaw(ctx, id)
						if err != nil {
							errs <- err
							return
						}
						if len(insp.Spec.TaskTemplate.Networks) > 0 {
							ret <- insp
							return
						}
						time.Sleep(1000 * time.Millisecond)
					}
				}()
				return ret, errs
			}(got.ID)

			result := swarm.Service{}
			select {
			case <-timeoutCtx.Done():
				t.Errorf("timeout context exceeded")
				return
			case <-errs:
				t.Errorf("%v", err)
				return
			case r := <-res:
				result = r
			}

			assert.Equal(t, tt.want.Spec.Annotations.Name, result.Spec.Annotations.Name)
			assert.Equal(t, tt.want.Spec.TaskTemplate.ContainerSpec.Image, result.Spec.TaskTemplate.ContainerSpec.Image)
			assert.Equal(t, tt.want.Spec.TaskTemplate.ContainerSpec.Env, result.Spec.TaskTemplate.ContainerSpec.Env)
			assert.Equal(t, tt.want.Spec.TaskTemplate.Networks[0].Target, result.Spec.TaskTemplate.Networks[0].Target)
			assert.Equal(t, tt.want.Spec.EndpointSpec, result.Spec.EndpointSpec)
			generatedIDs[i] = got.ID
		})
	}

	test.KillService(ctx, dockerClient, brainID)

	//Docker cleanup
	if err := test.DockerCleanUp(ctx, dockerClient, netID); err != nil {
		t.Errorf("cleanup error: %v", err)
	}
	os.Setenv("STAGE", env)
}

func Test_generateServiceSpec(t *testing.T) {
	var (
		maxAttempts = uint64(3)
		replicas    = uint64(1)
		second      = time.Second
	)

	tag := os.Getenv("TAG")
	if tag == "" {
		tag = "latest"
	}

	type args struct {
		config *PluginServiceConfig
	}
	tests := []struct {
		name    string
		args    args
		want    *swarm.ServiceSpec
		wantErr bool
		err     error
	}{
		{
			name: "Good config",
			args: args{
				config: &PluginServiceConfig{
					Address: GetManagerIP(),
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=666",
						"PLUGIN=GoodPlugin",
					},
					Network: "goodnet",
					OS:      "all",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    666,
						PublishedPort: 666,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "GoodService",
				},
			},
			want: &swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "GoodService",
				},
				TaskTemplate: swarm.TaskSpec{
					ContainerSpec: swarm.ContainerSpec{
						DNSConfig: &swarm.DNSConfig{},
						Env: []string{
							"STAGE=DEV",
							"LOGLEVEL=DEBUG",
							"PORT=666",
							"PLUGIN=GoodPlugin",
							"RETHINK_HOST=" + GetManagerIP(),
						},
						Healthcheck: &container.HealthConfig{
							Interval: time.Second,
							Timeout:  time.Second * 3,
							Retries:  3,
						},
						Image:           "ramrodpcp/interpreter-plugin:" + tag,
						StopGracePeriod: &second,
					},
					RestartPolicy: &swarm.RestartPolicy{
						Condition:   "on-failure",
						MaxAttempts: &maxAttempts,
					},
					Placement: &swarm.Placement{
						Constraints: []string{
							"node.labels.os==posix",
							"node.labels.ip==" + GetManagerIP(),
						},
					},
					Networks: []swarm.NetworkAttachmentConfig{
						swarm.NetworkAttachmentConfig{
							Target: "goodnet",
						},
					},
				},
				Mode: swarm.ServiceMode{
					Replicated: &swarm.ReplicatedService{
						Replicas: &replicas,
					},
				},
				EndpointSpec: &swarm.EndpointSpec{
					Mode: swarm.ResolutionModeVIP,
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    666,
						PublishedPort: 666,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
				},
			},
			wantErr: false,
		},
		{
			name: "Good config (win)",
			args: args{
				config: &PluginServiceConfig{
					Address: GetManagerIP(),
					Environment: []string{
						"STAGE=PROD",
						"LOGLEVEL=DEBUG",
						"PORT=777",
						"PLUGIN=GoodPluginWin",
					},
					Network: "goodnet",
					OS:      "nt",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolUDP,
						TargetPort:    777,
						PublishedPort: 777,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "GoodServiceWin",
				},
			},
			want: &swarm.ServiceSpec{
				Annotations: swarm.Annotations{
					Name: "GoodServiceWin",
				},
				TaskTemplate: swarm.TaskSpec{
					ContainerSpec: swarm.ContainerSpec{
						DNSConfig: &swarm.DNSConfig{},
						Env: []string{
							"STAGE=PROD",
							"LOGLEVEL=DEBUG",
							"PORT=777",
							"PLUGIN=GoodPluginWin",
							"RETHINK_HOST=" + GetManagerIP(),
						},
						Healthcheck: &container.HealthConfig{
							Interval: time.Second,
							Timeout:  time.Second * 3,
							Retries:  3,
						},
						Image:           "ramrodpcp/interpreter-plugin-windows:" + tag,
						StopGracePeriod: &second,
						Hosts:           []string{hostString("rethinkdb", GetManagerIP())},
					},
					RestartPolicy: &swarm.RestartPolicy{
						Condition:   "on-failure",
						MaxAttempts: &maxAttempts,
					},
					Placement: &swarm.Placement{
						Constraints: []string{
							"node.labels.os==nt",
							"node.labels.ip==" + GetManagerIP(),
						},
					},
					Networks: []swarm.NetworkAttachmentConfig{
						swarm.NetworkAttachmentConfig{
							Target: "goodnet",
						},
					},
				},
				Mode: swarm.ServiceMode{
					Replicated: &swarm.ReplicatedService{
						Replicas: &replicas,
					},
				},
				EndpointSpec: &swarm.EndpointSpec{
					Mode: swarm.ResolutionModeVIP,
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolUDP,
						TargetPort:    777,
						PublishedPort: 777,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
				},
			},
			wantErr: false,
		},
		{
			name: "Bad config OS",
			args: args{
				config: &PluginServiceConfig{
					Address: GetManagerIP(),
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=666",
						"PLUGIN=GoodPlugin",
					},
					Network: "goodnet",
					OS:      "dumb",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    666,
						PublishedPort: 666,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "BadService",
				},
			},
			want:    &swarm.ServiceSpec{},
			wantErr: true,
			err:     errors.New("invalid OS setting: dumb"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateServiceSpec(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateServiceSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				assert.Equal(t, tt.err, err)
				assert.Equal(t, tt.want, got)
				return
			}

			assert.Equal(t, tt.want.Annotations.Name, got.Annotations.Name)
			assert.Equal(t, *tt.want.TaskTemplate.ContainerSpec.DNSConfig, *got.TaskTemplate.ContainerSpec.DNSConfig)
			assert.Equal(t, tt.want.TaskTemplate.ContainerSpec.Env, got.TaskTemplate.ContainerSpec.Env)
			assert.Equal(t, *tt.want.TaskTemplate.ContainerSpec.Healthcheck, *got.TaskTemplate.ContainerSpec.Healthcheck)
			assert.Equal(t, tt.want.TaskTemplate.ContainerSpec.Image, got.TaskTemplate.ContainerSpec.Image)
			assert.Equal(t, *tt.want.TaskTemplate.ContainerSpec.StopGracePeriod, *got.TaskTemplate.ContainerSpec.StopGracePeriod)
			assert.Equal(t, *tt.want.TaskTemplate.RestartPolicy, *got.TaskTemplate.RestartPolicy)
			assert.Equal(t, *tt.want.TaskTemplate.Placement, *got.TaskTemplate.Placement)
			assert.Equal(t, *tt.want.Mode.Replicated.Replicas, *got.Mode.Replicated.Replicas)
			assert.Equal(t, tt.want.EndpointSpec.Mode, got.EndpointSpec.Mode)
			assert.Equal(t, tt.want.EndpointSpec.Ports, got.EndpointSpec.Ports)
		})
	}
}

func Test_getTagFromEnv(t *testing.T) {
	oldEnvTag := os.Getenv("TAG")

	tests := []struct {
		name   string
		setEnv []string
		want   string
	}{
		{
			name:   "Default not set",
			setEnv: []string{"TAG", ""},
			want:   "latest",
		},
		{
			name:   "TAG dev",
			setEnv: []string{"TAG", "dev"},
			want:   "dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TAG", "")
			os.Setenv(tt.setEnv[0], tt.setEnv[1])
			got := getTagFromEnv()
			assert.Equal(t, tt.want, got)
		})
	}
	os.Setenv("TAG", oldEnvTag)
}

func Test_GetManagerIP(t *testing.T) {

	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	nodeInspect, err := dockerClient.NodeList(ctx, types.NodeListOptions{})
	nodeIP := nodeInspect[0].Status.Addr

	tests := []struct {
		name string
		want string
	}{
		{
			name: "get ip",
			want: nodeIP,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetManagerIP(); got != tt.want {
				t.Errorf("GetManagerIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hostString(t *testing.T) {
	type args struct {
		h string
		i string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test 1",
			args: args{
				h: "rethinkdb",
				i: "127.0.0.1",
			},
			want: "rethinkdb:127.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hostString(tt.args.h, tt.args.i); got != tt.want {
				t.Errorf("hostString() = %v, want %v", got, tt.want)
			}
		})
	}
}
