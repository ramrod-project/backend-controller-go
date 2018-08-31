package helper

import "context"

func TimeoutTester(ctx context.Context, args []interface{}, f func(args ...interface{}) bool) <-chan bool {
	done := make(chan bool)

	go func() {
		for {
			recv := make(chan bool)

			go func() {
				recv <- f(args...)
				close(recv)
				return
			}()

			select {
			case <-ctx.Done():
				done <- false
				close(done)
				return
			case b := <-recv:
				done <- b
				close(done)
				return
			}
		}
	}()

	return done
}
