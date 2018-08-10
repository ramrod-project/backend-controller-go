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

	// Start the event monitor

	// Start event handler

	// Start the plugin database change monitor
	pluginData, pluginErr := rethink.MonitorPlugins()

	// Start the plugin action handler
	actionErr := dockerservicemanager.HandlePluginChanges(pluginData)

	// Monitor all errors in the main loop
	errChan := errorhandler.ErrorHandler(pluginErr, actionErr)

	for err := range errChan {
		if err != nil {
			log.Printf("Error: %v\n", err)
		}
	}
}
