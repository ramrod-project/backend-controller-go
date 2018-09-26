package dockerservicemanager

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	container "github.com/docker/docker/api/types/container"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/test"
	"github.com/stretchr/testify/assert"
	r "gopkg.in/gorethink/gorethink.v4"
)

func sliceEqual(slice1 *[]string, slice2 *[]string) bool {
	for _, e := range *slice1 {
		for _, a := range *slice2 {
			if a == e {
				return true
			}
		}
	}
	return false
}

func TestUpdatePluginService(t *testing.T) {
	env := os.Getenv("STAGE")
	os.Setenv("STAGE", "TESTING")
	var (
		maxAttempts     = uint64(3)
		placementConfig = &swarm.Placement{}
		replicas        = uint64(1)
		second          = time.Second
	)

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
		"NodeHostName": "ubuntu",
		"OS":           "posix",
	}
	_, err = r.DB("Controller").Table("Ports").Insert(testPort).RunWrite(session)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	netID, err := test.CheckCreateNet("test_update")
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	serviceSpec := &swarm.ServiceSpec{
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
					"PLUGIN=Harness",
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
			Placement: placementConfig,
			Networks: []swarm.NetworkAttachmentConfig{
				swarm.NetworkAttachmentConfig{
					Target: "test_update",
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
				PublishMode:   swarm.PortConfigPublishModeHost,
			}},
		},
	}

	serviceExtraSpec := &swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: "GoodServiceExtra",
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				DNSConfig: &swarm.DNSConfig{},
				Env: []string{
					"STAGE=DEV",
					"LOGLEVEL=DEBUG",
					"PORT=667",
					"PLUGIN=Harness",
					"RETHINK_HOST=" + GetManagerIP(),
				},
				Healthcheck: &container.HealthConfig{
					Interval: time.Second,
					Timeout:  time.Second * 3,
					Retries:  3,
				},
				Image:           "ramrodpcp/interpreter-plugin-extra:" + tag,
				StopGracePeriod: &second,
			},
			RestartPolicy: &swarm.RestartPolicy{
				Condition:   "on-failure",
				MaxAttempts: &maxAttempts,
			},
			Placement: placementConfig,
			Networks: []swarm.NetworkAttachmentConfig{
				swarm.NetworkAttachmentConfig{
					Target: "test_update",
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
				TargetPort:    667,
				PublishedPort: 667,
				PublishMode:   swarm.PortConfigPublishModeHost,
			}},
		},
	}

	resp, err := dockerClient.ServiceCreate(ctx, *serviceSpec, types.ServiceCreateOptions{})
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	id := resp.ID

	respExtra, err := dockerClient.ServiceCreate(ctx, *serviceExtraSpec, types.ServiceCreateOptions{})
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	extraID := respExtra.ID

	type args struct {
		config *PluginServiceConfig
		id     string
	}
	tests := []struct {
		name       string
		args       args
		want       types.ServiceUpdateResponse
		wantErr    bool
		inspectRes swarm.Service
	}{
		{
			name: "Valid service update",
			args: args{
				config: &PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=666",
						"PLUGIN=Harness",
						"TEST=TEST",
						"PLUGIN_NAME=GoodService",
					},
					Network: "test_update",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    666,
						PublishedPort: 666,
						PublishMode:   swarm.PortConfigPublishModeHost,
					}},
					ServiceName: "GoodService",
					Address:     GetManagerIP(),
				},
				id: id,
			},
			inspectRes: swarm.Service{
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name: "GoodService",
					},
					TaskTemplate: swarm.TaskSpec{
						ContainerSpec: swarm.ContainerSpec{
							Image: "ramrodpcp/interpreter-plugin:" + getTagFromEnv(),
							Env: []string{
								"STAGE=DEV",
								"LOGLEVEL=DEBUG",
								"PORT=666",
								"PLUGIN=Harness",
								"PLUGIN_NAME=GoodService",
								"RETHINK_HOST=" + GetManagerIP(),
								"TEST=TEST",
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
								TargetPort:    uint32(666),
								PublishedPort: uint32(666),
								PublishMode:   swarm.PortConfigPublishModeHost,
							},
						},
					},
				},
			},
			want: types.ServiceUpdateResponse{
				Warnings: nil,
			},
			wantErr: false,
		},
		{
			name: "Valid service update (extra)",
			args: args{
				config: &PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=667",
						"PLUGIN=Harness",
						"PLUGIN_NAME=GoodServiceExtra",
						"TEST=TEST",
					},
					Network: "test_update",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolUDP,
						TargetPort:    667,
						PublishedPort: 667,
						PublishMode:   swarm.PortConfigPublishModeHost,
					}},
					ServiceName: "GoodServiceExtra",
					Address:     GetManagerIP(),
					Extra:       true,
				},
				id: extraID,
			},
			inspectRes: swarm.Service{
				Spec: swarm.ServiceSpec{
					Annotations: swarm.Annotations{
						Name: "GoodServiceExtra",
					},
					TaskTemplate: swarm.TaskSpec{
						ContainerSpec: swarm.ContainerSpec{
							Image: "ramrodpcp/interpreter-plugin-extra:" + getTagFromEnv(),
							Env: []string{
								"STAGE=DEV",
								"LOGLEVEL=DEBUG",
								"PORT=667",
								"PLUGIN=Harness",
								"PLUGIN_NAME=GoodServiceExtra",
								"RETHINK_HOST=" + GetManagerIP(),
								"TEST=TEST",
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
								Protocol:      swarm.PortConfigProtocolUDP,
								TargetPort:    uint32(667),
								PublishedPort: uint32(667),
								PublishMode:   swarm.PortConfigPublishModeHost,
							},
						},
					},
				},
			},
			want: types.ServiceUpdateResponse{
				Warnings: nil,
			},
			wantErr: false,
		},
		{
			name: "Bad ID",
			args: args{
				config: &PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=5000",
						"PLUGIN=Harness",
						"TEST=TEST",
					},
					Network: "test_update",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    666,
						PublishedPort: 666,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "GoodService",
					Address:     GetManagerIP(),
				},
				id: "",
			},
			want: types.ServiceUpdateResponse{
				Warnings: nil,
			},
			wantErr: true,
		},
		{
			name: "Invalid service name change",
			args: args{
				config: &PluginServiceConfig{
					Environment: []string{
						"STAGE=DEV",
						"LOGLEVEL=DEBUG",
						"PORT=5000",
						"PLUGIN=Harness",
						"TEST=TEST",
					},
					Network: "test_update",
					OS:      "posix",
					Ports: []swarm.PortConfig{swarm.PortConfig{
						Protocol:      swarm.PortConfigProtocolTCP,
						TargetPort:    0,
						PublishedPort: 0,
						PublishMode:   swarm.PortConfigPublishModeIngress,
					}},
					ServiceName: "BadServiceUpdate",
					Address:     GetManagerIP(),
				},
				id: id,
			},
			want: types.ServiceUpdateResponse{
				Warnings: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			time.Sleep(time.Second)
			got, err := UpdatePluginService(tt.args.id, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdatePluginService() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if tt.wantErr {
				return
			}

			assert.Equal(t, tt.want, got)

			timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			res, errs := func(checkID string) (<-chan swarm.Service, <-chan error) {
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
						insp, _, err := dockerClient.ServiceInspectWithRaw(ctx, checkID)
						if err != nil {
							errs <- err
							return
						}
						if *insp.Spec.Mode.Replicated.Replicas > 0 {
							ret <- insp
							return
						}
						time.Sleep(1000 * time.Millisecond)
					}
				}()
				return ret, errs
			}(tt.args.id)

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

			assert.Equal(t, tt.inspectRes.Spec.Annotations.Name, result.Spec.Annotations.Name)
			assert.Equal(t, tt.inspectRes.Spec.TaskTemplate.ContainerSpec.Image, result.Spec.TaskTemplate.ContainerSpec.Image)
			assert.True(t, sliceEqual(&tt.inspectRes.Spec.TaskTemplate.ContainerSpec.Env, &result.Spec.TaskTemplate.ContainerSpec.Env))
			assert.Equal(t, tt.inspectRes.Spec.TaskTemplate.Networks[0].Target, result.Spec.TaskTemplate.Networks[0].Target)
			assert.Equal(t, tt.inspectRes.Spec.EndpointSpec, result.Spec.EndpointSpec)
		})
	}

	test.KillService(ctx, dockerClient, brainID)

	//Docker cleanup
	if err := test.DockerCleanUp(ctx, dockerClient, netID); err != nil {
		t.Errorf("cleanup error: %v", err)
	}

	os.Setenv("STAGE", env)
}

func Test_checkReady(t *testing.T) {
	type args struct {
		ctx          context.Context
		dockerClient *client.Client
		serviceID    string
	}
	tests := []struct {
		name    string
		args    args
		want    uint64
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkReady(tt.args.ctx, tt.args.dockerClient, tt.args.serviceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkReady() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("checkReady() = %v, want %v", got, tt.want)
			}
		})
	}
}
