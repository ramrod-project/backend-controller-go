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

	fmt.Printf("[")
	for resp := range eventChan {
		if resp.Type == "service" {
			jresp, err := json.Marshal(resp)
			if err == nil {
				var out bytes.Buffer
				json.Indent(&out, jresp, "=", "\t")
				fmt.Printf("\n%s,\n", out)
			}
		}
	}
}
