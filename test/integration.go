package test

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func Test_Integration(t *testing.T) {

	ctx := context.TODO()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	netRes, err := dockerClient.NetworkCreate(ctx, "pcp", types.NetworkCreate{
		Driver:     "overlay",
		Attachable: true,
	})
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	// Start the controller service and rethinkdb

	tests := []struct {
		name string
	}{
		// Here be tests
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Do stuff
		})
	}
	//Docker cleanup
	if err := DockerCleanUp(ctx, dockerClient, netRes.ID); err != nil {
		t.Errorf("cleanup error: %v", err)
	}
}
