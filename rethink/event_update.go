package rethink

import (
	"fmt"
	"log"

	events "github.com/docker/docker/api/types/events"
	r "gopkg.in/gorethink/gorethink.v4"
)

func handleEvent(event events.Message, session *r.Session) (r.WriteResponse, error) {
	log.Printf("Event received: %v", event)
	filter := make(map[string]string)
	update := make(map[string]string)
	if v, ok := event.Actor.Attributes["name"]; ok {
		filter["ServiceName"] = v
	} else {
		return r.WriteResponse{}, fmt.Errorf("no Name Attribute")
	}
	if val, ok := event.Actor.Attributes["updatestate.new"]; ok {
		update["DesiredState"] = ""
		if val == "updating" {
			update["State"] = "Restarting"
		} else if val == "completed" {
			update["State"] = "Active"
		}
		res, err := r.DB("Controller").Table("Plugins").Filter(filter).Update(update).RunWrite(session)
		return res, err
	} else if event.Action == "create" {
		update["State"] = "Active"
		update["ServiceID"] = event.Actor.ID
		update["DesiredState"] = ""
		res, err := r.DB("Controller").Table("Plugins").Filter(filter).Update(update).RunWrite(session)
		return res, err
	} else if event.Action == "remove" {
		update["State"] = "Stopped"
		update["DesiredState"] = ""
		res, err := r.DB("Controller").Table("Plugins").Filter(filter).Update(update).RunWrite(session)
		return res, err
	}
	return r.WriteResponse{}, nil
}

// EventUpdate consumes the event channel from the docker
// client event monitor. If handles events (one by one at
// the moment) and updates the database as they are recieved.
func EventUpdate(in <-chan events.Message) (<-chan r.WriteResponse, <-chan error) {
	outErr := make(chan error)
	outDB := make(chan r.WriteResponse)

	session, err := r.Connect(r.ConnectOpts{
		Address: getRethinkHost(),
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
