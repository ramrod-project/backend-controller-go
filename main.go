package main

import (
	"log"

	"github.com/ramrod-project/backend-controller-go/dockerservicemanager"
	"github.com/ramrod-project/backend-controller-go/errorhandler"
	"github.com/ramrod-project/backend-controller-go/rethink"
)

func main() {
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
			log.Fatalf("Error: %v\n", err)
		}
	}
}
