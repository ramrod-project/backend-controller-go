package errorhandler

import (
	"log"
)

func ErrorHandler(errorChans ...<-chan error) {
	collector := make(chan error)

	for _, errChan := range errorChans {
		go func(c <-chan error) {
			for e := range c {
				collector <- e
			}
		}(errChan)
	}

	for err := range collector {
		if err != nil {
			log.Printf("Error: %v\n", err)
		}
	}
}
