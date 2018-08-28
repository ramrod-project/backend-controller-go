package rethink

import (
	"fmt"

	events "github.com/docker/docker/api/types/events"
	r "gopkg.in/gorethink/gorethink.v4"
)

func updatePluginStatus(serviceName string, update map[string]string) error {
	if serviceName == "" {
		return fmt.Errorf("cannot update without valid ServiceName")
	}

	filter := map[string]string{"ServiceName": serviceName}

	session, err := r.Connect(r.ConnectOpts{
		Address: GetRethinkHost(),
	})
	if err != nil {
		return err
	}

	res, err := r.DB("Controller").Table("Plugins").Filter(filter).Update(update).RunWrite(session)
	if res.Errors > 0 || !(res.Replaced > 0 || res.Updated > 0) {
		return fmt.Errorf("no plugin to update")
	}
	return err
}

func handleContainer(event events.Message) (string, map[string]string, error) {
	var serviceName string
	update := make(map[string]string)

	if _, ok := event.Actor.Attributes["com.docker.swarm.service.name"]; !ok {
		return "", update, fmt.Errorf("no container 'com.docker.swarm.service.name' Attribute")
	}
	serviceName = event.Actor.Attributes["com.docker.swarm.service.name"]
	if event.Action == "health_status: healthy" || event.Status == "health_status: healthy" {
		update["State"] = "Active"
		update["ServiceID"] = event.Actor.Attributes["com.docker.swarm.service.id"]
		update["DesiredState"] = ""
		return serviceName, update, nil
	} else if event.Action == "die" || event.Action == "health_status: unhealthy" || event.Status == "health_status: unhealthy" {
		update["State"] = "Stopped"
		update["DesiredState"] = ""
		return serviceName, update, nil
	}
	return "", update, fmt.Errorf("unhandled container event: %v", event.Action)
}

func handleService(event events.Message) (string, map[string]string, error) {
	var serviceName string
	update := make(map[string]string)

	if _, ok := event.Actor.Attributes["name"]; !ok {
		return "", update, fmt.Errorf("no service 'name' Attribute")
	}
	serviceName = event.Actor.Attributes["name"]
	if val, ok := event.Actor.Attributes["updatestate.new"]; ok && val == "updating" {
		update["DesiredState"] = ""
		update["State"] = "Restarting"
		return serviceName, update, nil
	} else if event.Actor.Attributes["os"] == "nt" && event.Action == "create" { // Special Windows case
		update["DesiredState"] = ""
		update["State"] = "Active"
		return serviceName, update, nil
	} else if event.Actor.Attributes["os"] == "nt" && event.Action == "remove" { // Special Windows case
		update["DesiredState"] = ""
		update["State"] = "Stopped"
		return serviceName, update, nil
	}
	return "", update, fmt.Errorf("unhandled service event: %v", event.Action)
}

// EventUpdate consumes the event channel from the docker
// client event monitor. If handles events (one by one at
// the moment) and updates the database as they are recieved.
func EventUpdate(in <-chan events.Message) <-chan error {
	outErr := make(chan error)

	go func(in <-chan events.Message) {
	L:
		for event := range in {
			var (
				err         error
				serviceName string
				update      map[string]string
			)
			switch event.Type {
			case "service":
				// Check if updatestatus.new == updating
				serviceName, update, err = handleService(event)
			case "container":
				// Check if health_status == healthy
				// Check if event == die or health_status == unhealthy
				serviceName, update, err = handleContainer(event)
			default:
				outErr <- fmt.Errorf("not container or service type")
				continue L
			}
			if err != nil {
				outErr <- err
				continue L
			}
			err = updatePluginStatus(serviceName, update)
			if err != nil {
				outErr <- err
			}
		}
	}(in)

	return outErr
}
