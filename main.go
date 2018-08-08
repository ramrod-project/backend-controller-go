package main

import (
	"fmt"

	dockerservicemanager "github.com/ramrod-project/backend-controller-go/dockerservicemanager"
	"github.com/ramrod-project/backend-controller-go/errorhandler"
	"github.com/ramrod-project/backend-controller-go/rethink"
	"encoding/json"
)

func main() {

	eventChan, dockError := dockerservicemanager.EventMonitor()

	fromDB, dbError := rethink.EventUpdate(eventChan)

	go errorhandler.ErrorHandler(dbError, dockError)

	for resp := range fromDB {
		out, err := json.Marshal(resp)
		if err != nil {
			fmt.Printf("DB response: %v\n\n", resp)
		}
	}
}
