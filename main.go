package main

import (
	"fmt"

	dockerservicemanager "github.com/manziman/backend-controller-go/dockerservicemanager"
	"github.com/manziman/backend-controller-go/errorhandler"
	"github.com/manziman/backend-controller-go/rethink"
)

func main() {

	eventChan, dockError := dockerservicemanager.EventMonitor()

	fromDB, dbError := rethink.EventUpdate(eventChan)

	go errorhandler.ErrorHandler(dbError, dockError)

	for resp := range fromDB {
		fmt.Printf("DB response: %v\n", resp)
	}
}
