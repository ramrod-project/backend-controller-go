package rethink

import (
	"fmt"

	events "github.com/docker/docker/api/types/events"
	r "gopkg.in/gorethink/gorethink.v4"
)

func handleEvent(event events.Message, session *r.Session) error {
	filter := make(map[string]string)
	update := make(map[string]string)
	if v, ok := event.Actor.Attributes["name"]; ok {
		filter["ServiceName"] = v
	} else {
		return fmt.Errorf("no Name Attribute")
	}
	if val, ok := event.Actor.Attributes["updatestate.new"]; ok {
		update["DesiredState"] = ""
		if val == "updating" {
			update["State"] = "Restarting"
		} else if val == "completed" {
			update["State"] = "Active"
		}
		_, err := r.DB("Controller").Table("Plugins").Filter(filter).Update(update).RunWrite(session)
		return err
	} else if event.Action == "create" {
		update["State"] = "Active"
		update["ServiceID"] = event.Actor.ID
		update["DesiredState"] = ""
		_, err := r.DB("Controller").Table("Plugins").Filter(filter).Update(update).RunWrite(session)
		return err
	} else if event.Action == "remove" {
		update["State"] = "Stopped"
		update["DesiredState"] = ""
		_, err := r.DB("Controller").Table("Plugins").Filter(filter).Update(update).RunWrite(session)
		return err
	}
	return nil
}

// EventUpdate consumes the event channel from the docker
// client event monitor. If handles events (one by one at
// the moment) and updates the database as they are recieved.
func EventUpdate(in <-chan events.Message) <-chan error {
	outErr := make(chan error)

	session, err := r.Connect(r.ConnectOpts{
		Address: getRethinkHost(),
	})
	if err != nil {
		panic(err)
	}

	go func(in <-chan events.Message) {
		for event := range in {
			err := handleEvent(event, session)
			if err != nil {
				outErr <- err
			}
		}
	}(in)

	return outErr
}
