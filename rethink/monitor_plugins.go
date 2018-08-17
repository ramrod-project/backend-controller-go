package rethink

import (
	"fmt"
	"log"
	"os"

	r "gopkg.in/gorethink/gorethink.v4"
)

// Plugin represents a plugin entry
// in the database.
type Plugin struct {
	Name          string
	ServiceID     string
	ServiceName   string
	DesiredState  PluginDesiredState `json:",omitempty"`
	State         PluginState        `json:",omitempty"`
	Address       string
	ExternalPorts []string
	InternalPorts []string
	OS            PluginOS
	Environment   []string
}

// PluginOS is the supported OS for the plugin
type PluginOS string

const (
	// PluginOSPosix is posix compoliant OS's.
	PluginOSPosix PluginOS = "posix"
	// PluginOSWindows is Windows.
	PluginOSWindows PluginOS = "nt"
	// PluginOSAll is any.
	PluginOSAll PluginOS = "all"
)

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

func GetRethinkHost() string {
	temp := os.Getenv("STAGE")
	if temp == "TESTING" {
		return "127.0.0.1"
	}
	return "rethinkdb"
}

func newPlugin(change map[string]interface{}) (*Plugin, error) {
	var (
		name        string
		serviceID   string
		serviceName string
		address     string
		desired     PluginDesiredState
		extports    []string
		intports    []string
		environment []string
		os          PluginOS
		state       PluginState
	)

	switch change["DesiredState"] {
	case string(DesiredStateActivate):
		desired = DesiredStateActivate
	case string(DesiredStateRestart):
		desired = DesiredStateRestart
	case string(DesiredStateStop):
		desired = DesiredStateStop
	case "":
		desired = DesiredStateNull
	default:
		return &Plugin{}, NewControllerError(fmt.Sprintf("invalid desired state %v sent", change["DesiredState"]))
	}

	switch change["State"] {
	case string(StateActive):
		state = StateActive
	case string(StateAvailable):
		state = StateAvailable
	case string(StateRestarting):
		state = StateRestarting
	case string(StateStopped):
		state = StateStopped
	default:
		return &Plugin{}, NewControllerError(fmt.Sprintf("invalid state %v sent", change["State"]))
	}

	switch change["OS"] {
	case string(PluginOSPosix):
		os = PluginOSPosix
	case string(PluginOSWindows):
		os = PluginOSWindows
	case string(PluginOSAll):
		os = PluginOSAll
	default:
		return &Plugin{}, NewControllerError(fmt.Sprintf("invalid OS setting %v sent", change["OS"]))
	}

	switch t := change["Name"].(type) {
	case string:
		name = change["Name"].(string)
		if name == "" {
			return &Plugin{}, NewControllerError("plugin name must not be blank")
		}
	case nil:
		return &Plugin{}, NewControllerError(fmt.Sprintf("plugin name must be string, is %v", t))
	}

	switch t := change["ServiceID"].(type) {
	case string:
		serviceID = change["ServiceID"].(string)
	default:
		return &Plugin{}, NewControllerError(fmt.Sprintf("plugin service id must be string, is %v", t))
	}

	switch t := change["ServiceName"].(type) {
	case string:
		serviceName = change["ServiceName"].(string)
		if serviceName == "" {
			return &Plugin{}, NewControllerError("service name must not be blank")
		}
	default:
		return &Plugin{}, NewControllerError(fmt.Sprintf("plugin service name must be string, is %v", t))
	}

	switch t := change["Interface"].(type) {
	case string:
		address = change["Interface"].(string)
	default:
		log.Printf("address %v of type %v passed", change["Interface"], t)
		address = ""
	}

	for _, v := range change["ExternalPorts"].([]interface{}) {
		extports = append(extports, v.(string))
	}

	for _, v := range change["InternalPorts"].([]interface{}) {
		intports = append(intports, v.(string))
	}

	if fmt.Sprintf("%T", change["Environment"]) == "[]string" {
		environment = change["Environment"].([]string)
	} else if fmt.Sprintf("%T", change["Environment"]) == "[]interface {}" {
		for _, v := range change["Environment"].([]interface{}) {
			environment = append(environment, v.(string))
		}
	} else {
		environment = []string{}
	}

	plugin := &Plugin{
		Name:          name,
		ServiceID:     serviceID,
		ServiceName:   serviceName,
		DesiredState:  desired,
		State:         state,
		Address:       address,
		ExternalPorts: extports,
		InternalPorts: intports,
		OS:            os,
		Environment:   environment,
	}

	return plugin, nil
}

func watchChanges(res *r.Cursor) (<-chan Plugin, <-chan error) {
	out := make(chan Plugin)
	errChan := make(chan error)
	go func(cursor *r.Cursor) {
		var doc map[string]interface{}
		for cursor.Next(&doc) {
			if v, ok := doc["new_val"]; ok {
				plugin, err := newPlugin(v.(map[string]interface{}))
				if err != nil {
					errChan <- err
				}
				out <- *plugin
			}
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
		Address: GetRethinkHost(),
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
