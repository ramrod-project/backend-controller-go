package dockerservicemanager

import "context"

/*
Pseudocode:

NewLogFilter
filterArgs for: container start event of stack/plugin/aux images

stackContainerIDs
takes: nil
returns: []string (list of stack container IDs)

	query docker for list of services
	filter by stack members (com.whatever.something.stackid)
	return list of container IDs

NewLogMonitor
takes: context
returns: <-chan string (container IDs), <-chan error

	create channel for container IDs

	(goroutine)
		query docker for stack member logs
		push out stack member container IDs over channel
		get docker events channel (using NewLogFilter)
		while forever
			receive event
			parse contianer ID
			chan <- ID

	return channel

*/

func NewLogMonitor(ctx context.Context) (<-chan string, <-chan error) {
	ret := make(chan string)
	errs := make(chan error)
	return ret, errs
}
