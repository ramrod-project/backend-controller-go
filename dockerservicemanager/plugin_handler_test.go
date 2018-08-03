package dockerservicemanager

import (
	"reflect"
	"testing"

	rethink "github.com/manziman/backend-controller-go/rethink"
)

func TestHandlePluginChanges(t *testing.T) {
	type args struct {
		feed <-chan rethink.Plugin
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
			if got := HandlePluginChanges(tt.args.feed); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HandlePluginChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_selectChange(t *testing.T) {
	type args struct {
		plugin rethink.Plugin
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectChange(tt.args.plugin)
			if (err != nil) != tt.wantErr {
				t.Errorf("selectChange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("selectChange() = %v, want %v", got, tt.want)
			}
		})
	}
}
