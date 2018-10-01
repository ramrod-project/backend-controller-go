package dockerservicemanager

/*
Pseudocode:

NewLogHandler
takes: <-chan string (from log monitor)
returns: <-chan chan string (for the aggregator), <-chan errors
	create channels
	(goroutine)
		receive new ID
		create chan string
		spawn goroutine
		(goroutine)
			create log reader for the container
			read from log forever
			send log strings back over channel
		chan chan string <- chan string
	return channels
*/

func NewLogHandler(newIDs <-chan string) (<-chan chan string, <-chan error) {
	ret := make(chan chan string)
	errs := make(chan error)
	return ret, errs
}
