package rethink

import (
	"reflect"
	"testing"

	events "github.com/docker/docker/api/types/events"
	r "gopkg.in/gorethink/gorethink.v4"
)

func Test_handleEvent(t *testing.T) {
	type args struct {
		event   events.Message
		session *r.Session
	}
	tests := []struct {
		name    string
		args    args
		want    r.WriteResponse
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handleEvent(tt.args.event, tt.args.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("handleEvent() = %v, want %v", got, tt.want)
			}
		})
	}
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
