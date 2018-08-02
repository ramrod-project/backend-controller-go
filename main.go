package main

import (
	"fmt"

	dockerservicemanager "github.com/manziman/backend-controller-go/dockerservicemanager"
	rethink "github.com/manziman/backend-controller-go/rethink"
)

func main() {

	eventChan, errChan := dockerservicemanager.EventMonitor()

	fromDB, dbError := rethink.EventUpdate(eventChan)

	go func(in <-chan error) {
		for err := range in {
			if err != nil {
				panic(err)
			}
		}
	}(errChan)

	go func(in <-chan error) {
		for err := range in {
			if err != nil {
				panic(err)
			}
		}
	}(dbError)

	for resp := range fromDB {
		fmt.Printf("DB response: %v\n", resp)
	}
}
