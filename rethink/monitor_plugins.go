package rethink

import (
	"log"

	r "gopkg.in/gorethink/gorethink.v4"
)

// Plugin represents a plugin entry
// in the database.
type Plugin struct {
	Name          string
	ServiceName   string
	DesiredState  PluginDesiredState `json:",omitempty`
	State         PluginState        `json:",omitempty`
	Interface     string
	ExternalPorts []string
	InternalPorts []string
	OS            string
}

// PluginDesiredState is the desired state of the
// plugin service.
type PluginDesiredState string

const (
	// DesiredStateActivate is the start plugin command.
	DesiredStateActivate PluginDesiredState = "Activate"
	// DesiredStateRestart is the restart plugin command.
	DesiredStateRestart PluginDesiredState = "Restart"
	// DesiredStateStop is the stop plugin command.
	DesiredStateStop PluginDesiredState = "Stop"
	// DesiredStateNull is no command.
	DesiredStateNull PluginDesiredState = ""
)

// PluginState is the state of the plugin service.
type PluginState string

const (
	// StateAvailable is the plugin available state.
	StateAvailable PluginState = "Available"
	// StateActive is the running state.
	StateActive PluginState = "Active"
	// StateRestarting is the updating state.
	StateRestarting PluginState = "Restarting"
	// StateStopped is the removed state.
	StateStopped PluginState = "Stopped"
)

// NewControllerError returns a custom ControllerError.
func NewControllerError(text string) error {
	return &ControllerError{text}
}

// ControllerError is a custom error for the controller.
type ControllerError struct {
	s string
}

func (e *ControllerError) Error() string {
	return e.s
}

func newPlugin(change map[string]interface{}) (*Plugin, error) {
	var (
		desired PluginDesiredState
	)

	switch change["DesiredState"] {
	case DesiredStateActivate:
		desired = DesiredStateActivate
	case DesiredStateRestart:
		desired = DesiredStateRestart
	case DesiredStateStop:
		desired = DesiredStateStop
	case DesiredStateNull:
		desired = DesiredStateNull
	default:
		return &Plugin{}, NewControllerError("Invalid state sent!")
	}

	plugin := &Plugin{
		Name:          change["Name"].(string),
		ServiceName:   change["ServiceName"].(string),
		DesiredState:  desired,
		State:         change["State"].(PluginState),
		Interface:     change["Interface"].(string),
		ExternalPorts: change["ExternalPorts"].([]string),
		InternalPorts: change["InternalPorts"].([]string),
		OS:            change["OS"].(string),
	}

	return plugin, nil
}

func watchChanges(res *r.Cursor) (<-chan Plugin, <-chan error) {
	out := make(chan Plugin)
	errChan := make(chan error)
	go func(cursor *r.Cursor) {
		var doc map[string]interface{}
		for cursor.Next(&doc) {
			log.Printf("Change: %v, Type: %T", doc, doc)
			plugin, err := newPlugin(doc)
			if err != nil {
				errChan <- err
			}
			out <- *plugin
		}
	}(res)
	return out, errChan
}

// MonitorPlugins purpose of this function is to monitor changes
// in the Controller.Plugins table. It returns both a
// channel with the changes, as well as an error channel.
// The output channel is consumed by the routine(s)
// handling the changes to the state of the services.
// At some point the query here will be filtered down
// to only the changes that matter.
func MonitorPlugins() (<-chan Plugin, <-chan error) {
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

	outDB, errDB := watchChanges(res)

	return outDB, errDB
}
