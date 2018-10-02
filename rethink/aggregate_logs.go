package rethink

import (
	"context"

	"github.com/ramrod-project/backend-controller-go/customtypes"
	r "gopkg.in/gorethink/gorethink.v4"
)

/*
Pseudocode

dbLogQuery
	a query for the db logs table

logSend
takes: rethink connection, customtypes.ContainerLog
returns: error
	insert document into database

AggregateLogs
takes: context, <-chan <-chan customtypes.ContainerLog
returns: <-chan error
	make error chan
	(goroutine)
		defer close error chan
		make <-chan customtypes.ContainerLog list
		rethink SetTags("rethinkdb", "json")
		connect to db
		while forever
			check context done or new chan
			if new chan, append to chans
			for each channel
				if not readable remove from slice
				if readable and value not nil, logSend()
*/

var dbLogQuery = r.DB("Brain").Table("Logs")

func logSend(sess *r.Session, log customtypes.ContainerLog) error {

	if log == (customtypes.ContainerLog{}) {
		return nil
	}

	_, err := dbLogQuery.Update(log).RunWrite(sess)
	if err != nil {
		return err
	}
	return nil
}

// AggregateLogs takes a dynamic number of log
// channels and aggregates the output to send to
// the logs database.
func AggregateLogs(ctx context.Context, logChans <-chan (<-chan customtypes.ContainerLog)) <-chan error {
	errs := make(chan error)

	go func() {
		defer close(errs)

		logSlice := []<-chan customtypes.ContainerLog{}

		r.SetTags("rethinkdb", "json")

		session, err := r.Connect(r.ConnectOpts{
			Address: GetRethinkHost(),
		})
		if err != nil {
			errs <- err
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case c, ok := <-logChans:
				if !ok {
					return
				}
				logSlice = append(logSlice, c)
			}

			for i, c := range logSlice {
				if l, ok := <-c; !ok {
					logSlice = append(logSlice[:i], logSlice[i+1:]...)
					i--
					continue
				} else {
					err = logSend(session, l)
					if err != nil {
						errs <- err
					}
				}
			}
		}
	}()

	return errs
}
