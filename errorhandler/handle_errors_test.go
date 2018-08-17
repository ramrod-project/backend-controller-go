package errorhandler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestErrorHandler(t *testing.T) {

	tests := []struct {
		name       string
		errorFuncs []func(chan error)
		check      func(context.Context, *testing.T, <-chan error) bool
	}{
		{
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorChans := make([]<-chan error, len(tt.errorFuncs))
			for i, f := range tt.errorFuncs {
				errChan := make(chan error)
				go f(errChan)
				errorChans[i] = errChan
			}
			errs := ErrorHandler(errorChans...)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			assert.True(t, tt.check(ctx, t, errs))
		})
	}
}
