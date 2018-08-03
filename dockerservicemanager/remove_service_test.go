package dockerservicemanager

import "testing"

func TestRemovePluginService(t *testing.T) {
	type args struct {
		serviceID string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RemovePluginService(tt.args.serviceID); (err != nil) != tt.wantErr {
				t.Errorf("RemovePluginService() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
