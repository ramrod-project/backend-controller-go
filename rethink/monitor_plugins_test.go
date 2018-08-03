package rethink

import (
	"reflect"
	"testing"

	r "gopkg.in/gorethink/gorethink.v4"
)

func Test_newPlugin(t *testing.T) {
	type args struct {
		change map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    *Plugin
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newPlugin(tt.args.change)
			if (err != nil) != tt.wantErr {
				t.Errorf("newPlugin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newPlugin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_watchChanges(t *testing.T) {
	type args struct {
		res *r.Cursor
	}
	tests := []struct {
		name  string
		args  args
		want  <-chan Plugin
		want1 <-chan error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := watchChanges(tt.args.res)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("watchChanges() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("watchChanges() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestMonitorPlugins(t *testing.T) {
	tests := []struct {
		name  string
		want  <-chan Plugin
		want1 <-chan error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := MonitorPlugins()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MonitorPlugins() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("MonitorPlugins() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
