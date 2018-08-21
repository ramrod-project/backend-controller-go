package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/ramrod-project/backend-controller-go/dockerservicemanager"
	"github.com/ramrod-project/backend-controller-go/errorhandler"
	"github.com/ramrod-project/backend-controller-go/rethink"
	r "gopkg.in/gorethink/gorethink.v4"
)

func checkDB(timeout time.Duration) bool {
	// Verify db connection
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	done := func() <-chan struct{} {
		con := make(chan struct{})
		go func() {
			defer cancel()
			for {
				select {
				case <-ctx.Done():
					close(con)
					return
				default:
					break
				}
				session, err := r.Connect(r.ConnectOpts{
					Address: rethink.GetRethinkHost(),
				})
				if err == nil {
					_, err := r.DB("Controller").Table("Plugins").Run(session)
					if err == nil {
						con <- struct{}{}
						close(con)
						return
					}
				}
				time.Sleep(1000 * time.Millisecond)
			}
		}()
		return con
	}()
	select {
	case <-ctx.Done():
		<-done
		return false
	case <-done:
		log.Printf("success: brain connection verified")
		return true
	}
}

func main() {
	// Check the connection to the database before
	// doing anything.
	if !checkDB(10 * time.Second) {
		log.Fatalf("fatal: %v", errors.New("database connection attempt timed out, exiting"))
	}

	// Advertise nodes to database
	err := dockerservicemanager.NodeAdvertise()
	if err != nil {
		log.Fatalf("fatal: %v", err)
	}

	// Populate with plugin data from manifest and
	// update services.
	err = dockerservicemanager.PluginAdvertise()
	if err != nil {
		log.Fatalf("fatal: %v", err)
	}

	// Start up Harness and Aux if needed
	err = dockerservicemanager.StartupServices()
	if err != nil {
		log.Fatalf("fatal: %v", err)
	}

	log.Printf("success: advertisement complete without errors...")

	// Start the event monitor
	eventData, eventErr := dockerservicemanager.EventMonitor()

	log.Printf("success: event monitor started...")

	// Start event handler
	eventDBErr := rethink.EventUpdate(eventData)

	log.Printf("success: event handler started...")

	// Start the plugin database change monitor
	pluginData, pluginErr := rethink.MonitorPlugins()

	log.Printf("success: plugin monitor started...")

	// Start the plugin action handler
	actionErr := dockerservicemanager.HandlePluginChanges(pluginData)

	log.Printf("success: plugin handler started...")

	// Monitor all errors in the main loop
	errChan := errorhandler.ErrorHandler(pluginErr, actionErr, eventErr, eventDBErr)

	for err := range errChan {
		if err != nil {
			log.Printf("error: %v\n", err)
		}
	}
}
