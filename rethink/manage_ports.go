package rethink

import (
	"errors"
	"log"

	r "gopkg.in/gorethink/gorethink.v4"
)

type Port struct {
	Interface   string
	TCPPorts    []string
	UDPPorts    []string
	NodHostName string
	OS          string
}

func contains(arr []string, element string) bool {
	for _, i := range arr {
		if i == element {
			return true
		}
	}
	return false
}

// AddPort adds a list of ports to the Ports table. it returns an error if
// there was a duplicate
func AddPort(IPaddr string, newPort string, protocol string) error {
	session, err := r.Connect(r.ConnectOpts{
		Address: getRethinkHost(),
	})
	if err != nil {
		panic(err)
	}
	//get the current entry
	filter := make(map[string]interface{})
	filter["Interface"] = IPaddr
	entry, _ := r.DB("Controller").Table("Ports").Filter(filter).Run(session)
	var port map[string]interface{}
	entry.Next(&port)
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
	_, err = r.DB("Controller").Table("Ports").Get(port["id"]).Update(newPort).RunWrite(session)
	if err != nil {
		log.Printf("%v", err)
		return err
	}
	return nil
}

func RemovePort(remPort string) error {
	// session, err := r.Connect(r.ConnectOpts{
	// 	Address: getRethinkHost(),
	// })
	// if err != nil {
	// 	panic(err)
	// }

	// filter := make(map[string]interface{})
	// portData := strings.Split(remPort, "/")
	// _, err = r.DB("Controller").Table("Ports").Filter().RunWrite(session)

	// if err != nil {
	// 	log.Printf("Port(s) not found")
	// 	return err
	// }
	return nil
}
