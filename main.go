package main

import (
	"bytes"
	"fmt"

	"encoding/json"

	dockerservicemanager "github.com/ramrod-project/backend-controller-go/dockerservicemanager"
	// "github.com/ramrod-project/backend-controller-go/errorhandler"
	// "github.com/ramrod-project/backend-controller-go/rethink"
)

func main() {
	// _ = dockError
	eventChan, _ := dockerservicemanager.EventMonitor()

	// fromDB, dbError := rethink.EventUpdate(eventChan)

	// go errorhandler.ErrorHandler(dbError, dockError)

	for resp := range eventChan {
		jresp, err := json.Marshal(resp)
		if err != nil {
			var out bytes.Buffer
			json.Indent(&out, jresp, "=", "\t")
			fmt.Printf("\nDB response: %s\n", out)
		}
	}
}
