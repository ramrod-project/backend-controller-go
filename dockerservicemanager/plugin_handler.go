package dockerservicemanager

import (
	rethink "github.com/manziman/backend-controller-go/rethink"
)

func selectChange(plugin rethink.Plugin) (interface{}, error) {
	if plugin.ServiceName == "" {
		return nil, nil
	}
	switch plugin.DesiredState {
	case rethink.DesiredStateActivate:
		print("%v %v", plugin.ServiceName, rethink.DesiredStateActivate)
	case rethink.DesiredStateRestart:
		print("%v %v", plugin.ServiceName, rethink.DesiredStateRestart)
	case rethink.DesiredStateStop:
		print("%v %v", plugin.ServiceName, rethink.DesiredStateStop)
	case rethink.DesiredStateNull:
		print("Do nothing")
	}
	return nil, nil
}

// HandlePluginChanges takes a channel of Plugins
// being fed by the plugin monitor routine and performs
// actions on their services as needed.
func HandlePluginChanges(feed <-chan rethink.Plugin) <-chan error {
	errChan := make(chan error)

	go func(in <-chan rethink.Plugin) {
		for plugin := range in {
			_, err := selectChange(plugin)
			if err != nil {
				errChan <- err
			}
		}
	}(feed)

	return errChan
}
