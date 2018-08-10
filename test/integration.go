package test

import (
	"context"
	"testing"

	"github.com/docker/docker/client"
)

func Test_Integration(t *testing.T) {

	ctx := context.TODO()
	dockerClient, err := client.NewEnvClient()
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
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Do stuff
		})
	}
	//Docker cleanup
	if err := dockerCleanUp(ctx, dockerClient, netRes.ID); err != nil {
		t.Errorf("cleanup error: %v", err)
	}
}
