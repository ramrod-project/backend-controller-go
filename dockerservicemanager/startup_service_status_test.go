package dockerservicemanager

import (
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	r "gopkg.in/gorethink/gorethink.v4"
)

func TestStartupServiceStatus(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := StartupServiceStatus(); (err != nil) != tt.wantErr {
				t.Errorf("StartupServiceStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_serviceToEntry(t *testing.T) {
	type args struct {
		s   *r.Session
		svc swarm.Service
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := serviceToEntry(tt.args.s, tt.args.svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("serviceToEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("serviceToEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}
