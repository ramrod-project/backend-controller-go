package dockerservicemanager

import (
	"reflect"
	"testing"

	events "github.com/docker/docker/api/types/events"
)

func TestEventMonitor(t *testing.T) {
	tests := []struct {
		name  string
		want  <-chan events.Message
		want1 <-chan error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := EventMonitor()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EventMonitor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("EventMonitor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
