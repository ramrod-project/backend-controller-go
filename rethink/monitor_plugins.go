package rethink

import (
	"log"

	r "gopkg.in/gorethink/gorethink.v4"
)

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
		panic(err)
	}

	outDB := watchChanges(res)

	return outDB, outErr
}
