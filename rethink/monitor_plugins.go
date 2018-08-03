package rethink

import (
	"log"

	r "gopkg.in/gorethink/gorethink.v4"
)

// Plugin represents a plugin entry
// in the database.
type Plugin interface{}

func watchChanges(res *r.Cursor) <-chan interface{} {
	out := make(chan interface{})
	go func(cursor *r.Cursor) {
		var doc interface{}
		for cursor.Next(&doc) {
			log.Printf("Change: %v", doc)
			out <- doc
		}
	}(res)
	return out
}

// MonitorPlugins purpose of this function is to monitor changes
// in the Controller.Plugins table. It returns both a
// channel with the changes, as well as an error channel.
// The output channel is consumed by the routine(s)
// handling the changes to the state of the services.
// At some point the query here will be filtered down
// to only the changes that matter.
func MonitorPlugins() (<-chan interface{}, <-chan error) {
	outErr := make(chan error)

	session, err := r.Connect(r.ConnectOpts{
		Address: "127.0.0.1",
	})
	if err != nil {
		panic(err)
	}

	res, err := r.DB("Controller").Table("Plugins").Changes().Run(session)
	if err != nil {
		outErr <- err
	}

	outDB := watchChanges(res)

	return outDB, outErr
}
