package main

import (
	"errors"
	"log"
	"time"

	"github.com/ramrod-project/backend-controller-go/dockerservicemanager"
	"github.com/ramrod-project/backend-controller-go/errorhandler"
	"github.com/ramrod-project/backend-controller-go/rethink"
	r "gopkg.in/gorethink/gorethink.v4"
)

func main() {

	// Check db
	start := time.Now()
	ready := false
	for {
		session, err := r.Connect(r.ConnectOpts{
			Address: rethink.GetRethinkHost(),
		})
		if err == nil {
			_, err := r.DB("Controller").Table("Plugins").Run(session)
			if err == nil {
				ready = true
				break
			}
		}
		if time.Since(start) >= 20*time.Second {
			panic(err)
		}
		time.Sleep(time.Second)
	}
	if !ready {
		panic(errors.New("Brain not ready"))
	}
	log.Printf("Brain ready...")

	// Advertise nodes to database
	err := dockerservicemanager.NodeAdvertise()
	if err != nil {
		panic(err)
	}

	// Populate with plugin data from manifest and
	// update services.
	err = dockerservicemanager.PluginAdvertise()
	if err != nil {
		panic(err)
	}

	// Start up Harness and Aux if needed
	err = dockerservicemanager.StartupServices()
	if err != nil {
		panic(err)
	}

	log.Printf("Advertisement complete without errors...")

	// Start the event monitor
	eventData, eventErr := dockerservicemanager.EventMonitor()

	log.Printf("Event monitor started...")

	// Start event handler
	eventDBErr := rethink.EventUpdate(eventData)

	log.Printf("Event handler started...")

	// Start the plugin database change monitor
	pluginData, pluginErr := rethink.MonitorPlugins()

	log.Printf("Plugin monitor started...")

	// Start the plugin action handler
	actionErr := dockerservicemanager.HandlePluginChanges(pluginData)

	log.Printf("Plugin handler started...")

	// Monitor all errors in the main loop
	errChan := errorhandler.ErrorHandler(pluginErr, actionErr, eventErr, eventDBErr)

	for err := range errChan {
		if err != nil {
			log.Printf("Error: %v\n", err)
		}
	}
}
