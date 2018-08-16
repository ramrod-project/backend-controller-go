package rethink

import (
	"errors"
	"log"

	r "gopkg.in/gorethink/gorethink.v4"
)

func contains(arr []string, element string) bool {
	for _, i := range arr {
		if i == element {
			return true
		}
	}
	return false
}

func remove(arr []string, element string) []string {
	if contains(arr, element) {
		for i, item := range arr {
			if item == element {
				return append(arr[:i], arr[i+1:]...)
			}
		}
	}
	return arr
}

func getCurrentEntry(IPaddr string, session *r.Session) map[string]interface{} {
	filter := make(map[string]interface{})
	filter["Interface"] = IPaddr
	entry, _ := r.DB("Controller").Table("Ports").Filter(filter).Run(session)
	var port map[string]interface{}
	entry.Next(&port)
	return port
}

// AddPort adds a port to the Ports table. it returns an error if
// there was a duplicate
func AddPort(IPaddr string, newPort string, protocol string) error {
	session, err := r.Connect(r.ConnectOpts{
		Address: getRethinkHost(),
	})
	if err != nil {
		panic(err)
	}
	//get the current entry
	var port map[string]interface{}
	port = getCurrentEntry(IPaddr, session)
	//update the ports
	if protocol == "tcp" {
		if contains(port["TCPPorts"].([]string), newPort) {
			return errors.New("port already in use")
		}
		port["TCPPorts"] = append(port["TCPPorts"].([]string), newPort)
	} else if protocol == "udp" {
		if contains(port["UDPPorts"].([]string), newPort) {
			return errors.New("port already in use")
		}
		port["UDPPorts"] = append(port["UDPPorts"].([]string), newPort)
	} else {
		return errors.New("only tcp and udp are supported protocols")
	}
	//update the entry
	_, err = r.DB("Controller").Table("Ports").Get(port["id"]).Update(port).RunWrite(session)
	if err != nil {
		log.Printf("%v", err)
		return err
	}
	return nil
}

// RemovePort removes a port to the Ports table. it returns an error if
// there was a duplicate
func RemovePort(IPaddr string, remPort string, protocol string) error {
	session, err := r.Connect(r.ConnectOpts{
		Address: getRethinkHost(),
	})
	if err != nil {
		panic(err)
	}

	// get current entry
	var port map[string]interface{}
	port = getCurrentEntry(IPaddr, session)
	// update ports
	if protocol == "tcp" {
		port["TCPPorts"] = remove(port["TCPPorts"].([]string), remPort)
	} else if protocol == "udp" {
		port["UDPPorts"] = remove(port["UDPPorts"].([]string), remPort)
	} else {
		return errors.New("only tcp and udp are supported protocols")
	}
	// update entry
	_, err = r.DB("Controller").Table("Ports").Get(port["id"]).Update(port).RunWrite(session)
	if err != nil {
		log.Printf("%v", err)
		return err
	}
	return nil
}
