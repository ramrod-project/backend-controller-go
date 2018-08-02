package rethink

import (
	"log"

	events "github.com/docker/docker/api/types/events"
	r "gopkg.in/gorethink/gorethink.v4"
)

func handleEvent(event events.Message, session *r.Session) (r.WriteResponse, error) {
	log.Printf("Event received: %v", event)
	if val, ok := event.Actor.Attributes["updatestate.new"]; ok {
		filter := make(map[string]string)
		update := make(map[string]string)
		filter["ServiceName"] = event.Actor.Attributes["name"]
		if val == "updating" {
			update["State"] = "Restarting"
		} else if val == "completed" {
			update["State"] = "Active"
		}
		res, err := r.DB("Controller").Table("Plugins").Filter(filter).Update(update).RunWrite(session)
		return res, err
	}
	return r.WriteResponse{}, nil
}

func EventUpdate(in <-chan events.Message) (<-chan r.WriteResponse, <-chan error) {
	outErr := make(chan error)
	outDB := make(chan r.WriteResponse)

	session, err := r.Connect(r.ConnectOpts{
		Address: "127.0.0.1",
	})
	if err != nil {
		panic(err)
	}

	go func(in <-chan events.Message) {
		for event := range in {
			response, err := handleEvent(event, session)
			outDB <- response
			outErr <- err
		}
	}(in)

	return outDB, outErr
}
