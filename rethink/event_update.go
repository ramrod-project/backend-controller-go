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
	} else if event.Action == "create" {
		insertion := make(map[string]string)
		insertion["ServiceName"] = event.Actor.Attributes["name"]
		insertion["ServiceID"] = event.Actor.ID
		r.DB("Controller").Table("plugins").Insert(insertion).RunWrite(session)
	} else if event.Action == "remove" {
		filter := make(map[string]string)
		update := make(map[string]string)
		filter["ServiceName"] = event.Actor.Attributes["name"]
		update["State"] = "Stopped"
		r.DB("Controller").Table("plugins").Filter(filter).Update(update).Run(session)
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
