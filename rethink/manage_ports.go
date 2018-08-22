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
	for i, item := range arr {
		if item == element {
			return append(arr[:i], arr[i+1:]...)
		}
	}
	return arr
}

func getCurrentEntry(IPaddr string, session *r.Session) map[string]interface{} {
	filter := make(map[string]interface{})
	filter["Interface"] = IPaddr
	log.Printf("\ngetting the data\n")
	entry, _ := r.DB("Controller").Table("Ports").Filter(filter).Run(session)
	var port map[string]interface{}
	entry.Next(&port)
	return port
}

// AddPort adds a port to the Ports table. it returns an error if
// there was a duplicate
func AddPort(IPaddr string, newPort string, protocol string) error {
	session, err := r.Connect(r.ConnectOpts{
		Address: "127.0.0.1",
	})
	if err != nil {
		return err
	}

	var (
		port   map[string]interface{}
		newTCP []string
		newUDP []string
	)
	log.Printf("\ngetting the entry\n")
	port = getCurrentEntry(IPaddr, session)

	for _, tcpPort := range port["TCPPorts"].([]interface{}) {
		newTCP = append(newTCP, tcpPort.(string))
	}
	for _, udpPort := range port["UDPPorts"].([]interface{}) {
		newUDP = append(newUDP, udpPort.(string))
	}

	//update the ports
	log.Printf("\nreplacing\n")
	if protocol == "tcp" {
		if contains(newTCP, newPort) {
			return errors.New("port already in use")
		}
		port["TCPPorts"] = append(newTCP, newPort)
	} else if protocol == "udp" {
		if contains(newUDP, newPort) {
			return errors.New("port already in use")
		}
		port["UDPPorts"] = append(newUDP, newPort)
	} else {
		return errors.New("only tcp and udp are supported protocols")
	}
	//update the entry
	log.Printf("\nupdating\n")
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
		Address: "127.0.0.1",
	})
	if err != nil {
		return err
	}

	// get current entry
	var (
		port   map[string]interface{}
		newTCP []string
		newUDP []string
	)
	port = getCurrentEntry(IPaddr, session)

	for _, tcpPort := range port["TCPPorts"].([]interface{}) {
		newTCP = append(newTCP, tcpPort.(string))
	}
	for _, udpPort := range port["UDPPorts"].([]interface{}) {
		newUDP = append(newUDP, udpPort.(string))
	}

	// update ports
	if protocol == "tcp" {
		port["TCPPorts"] = remove(newTCP, remPort)
	} else if protocol == "udp" {
		port["UDPPorts"] = remove(newUDP, remPort)
	} else {
		return errors.New("only tcp and udp are supported protocols")
	}
	// update entry
	resp, err := r.DB("Controller").Table("Ports").Get(port["id"]).Update(port).RunWrite(session)
	log.Printf("removed: %+v", resp)
	if err != nil {
		log.Printf("%v", err)
		return err
	}
	// if resp.Unchanged == 1 {
	// 	return errors.New("port doesn't exits")
	// }
	return nil
}
