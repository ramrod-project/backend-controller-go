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
		{
			name: "container healthy",
		},
		{
			name: "container unhealthy",
		},
		{
			name: "service update",
		},
		{
			name: "dont get anything",
		},
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

func Test_eventFanIn(t *testing.T) {
	type args struct {
		eventChans []<-chan events.Message
		errChans   []<-chan error
	}
	tests := []struct {
		name  string
		args  args
		want  <-chan events.Message
		want1 <-chan error
	}{
		/*{
			name: "One error channel",
			errorFuncs: []func(chan error){
				func(errs chan error) {
					time.Sleep(time.Second)
					errs <- errors.New("this is a test error")
					close(errs)
				},
			},
			check: func(ctx context.Context, t *testing.T, errs <-chan error) bool {

				select {
				case <-ctx.Done():
					return false
				case e := <-errs:
					assert.Equal(t, errors.New("this is a test error"), e)
					break
				}
				return true
			},
		},
		{
			name: "Two error channels",
			errorFuncs: []func(chan error){
				func(errs chan error) {
					time.Sleep(time.Second)
					for i := 0; i < 7; i++ {
						errs <- errors.New("func 1 err")
					}
					close(errs)
				},
				func(errs chan error) {
					time.Sleep(time.Second)
					for i := 0; i < 8; i++ {
						errs <- errors.New("func 2 err")
					}
					close(errs)
				},
			},
			check: func(ctx context.Context, t *testing.T, errs <-chan error) bool {
				count := 0
				select {
				case <-ctx.Done():
					return false
				case e := <-errs:
					assert.IsType(t, errors.New(""), e)
					count++
				default:
					if count >= 15 {
						return true
					}
				}
				return true
			},
		},*/
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := eventFanIn(tt.args.eventChans, tt.args.errChans)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("eventFanIn() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("eventFanIn() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
