package rethink

import (
	"errors"
	"fmt"
	"log"

	"github.com/docker/docker/api/types/swarm"
	r "gopkg.in/gorethink/gorethink.v4"
)

func GetIPFromID(servID string) (string, error) {
	session, err := r.Connect(r.ConnectOpts{
		Address: "127.0.0.1",
	})
	if err != nil {
		return "", err
	}
	filter := make(map[string]interface{})
	filter["ServiceID"] = servID
	entry, _ := r.DB("Controller").Table("Plugins").Filter(filter).Run(session)
	var res map[string]interface{}
	if !entry.Next(&res) {
		return "", err
	}
	addr := res["Interface"].(string)
	return addr, nil
}

func Contains(arr []string, element string) bool {
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

func getCurrentEntry(IPaddr string, session *r.Session) (map[string]interface{}, error) {
	filter := make(map[string]interface{})
	filter["Interface"] = IPaddr
	entry, _ := r.DB("Controller").Table("Ports").Filter(filter).Run(session)
	var port map[string]interface{}
	if !entry.Next(&port) {
		return port, fmt.Errorf("Interface not found: %v", IPaddr)
	}
	return port, nil
}

// AddPort adds a port to the Ports table. it returns an error if
// there was a duplicate
func AddPort(IPaddr string, newPort string, protocol swarm.PortConfigProtocol) error {
	session, err := r.Connect(r.ConnectOpts{
		Address: GetRethinkHost(),
	})
	if err != nil {
		return err
	}

	var (
		port   map[string]interface{}
		newTCP []string
		newUDP []string
	)
	port, err = getCurrentEntry(IPaddr, session)
	if err != nil {
		log.Printf("%v", err)
		return err
	}

	for _, tcpPort := range port["TCPPorts"].([]interface{}) {
		newTCP = append(newTCP, tcpPort.(string))
	}
	for _, udpPort := range port["UDPPorts"].([]interface{}) {
		newUDP = append(newUDP, udpPort.(string))
	}

	//update the ports
	if protocol == swarm.PortConfigProtocolTCP {
		if Contains(newTCP, newPort) {
			return errors.New("port already in use")
		}
		port["TCPPorts"] = append(newTCP, newPort)
	} else if protocol == swarm.PortConfigProtocolUDP {
		if Contains(newUDP, newPort) {
			return errors.New("port already in use")
		}
		port["UDPPorts"] = append(newUDP, newPort)
	} else {
		return errors.New("only tcp and udp are supported protocols")
	}
	//update the entry
	log.Printf("updating database with %+v\n", port)
	res, err := r.DB("Controller").Table("Ports").Get(port["id"]).Update(port).RunWrite(session)
	if err != nil {
		log.Printf("%v", err)
		return err
	}
	log.Printf("%+v\n", res)
	log.Printf("port added: %+v\n", port)
	return nil
}

// RemovePort removes a port to the Ports table. it returns an error if
// there was a duplicate
func RemovePort(IPaddr string, remPort string, protocol swarm.PortConfigProtocol) error {
	session, err := r.Connect(r.ConnectOpts{
		Address: GetRethinkHost(),
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
	port, err = getCurrentEntry(IPaddr, session)
	if err != nil {
		log.Printf("%v", err)
		return err
	}

	for _, tcpPort := range port["TCPPorts"].([]interface{}) {
		newTCP = append(newTCP, tcpPort.(string))
	}
	for _, udpPort := range port["UDPPorts"].([]interface{}) {
		newUDP = append(newUDP, udpPort.(string))
	}

	// update ports
	if protocol == swarm.PortConfigProtocolTCP {
		port["TCPPorts"] = remove(newTCP, remPort)
	} else if protocol == swarm.PortConfigProtocolUDP {
		port["UDPPorts"] = remove(newUDP, remPort)
	} else {
		return errors.New("only tcp and udp are supported protocols")
	}
	// update entry
	resp, err := r.DB("Controller").Table("Ports").Get(port["id"]).Update(port).RunWrite(session)
	if err != nil {
		log.Printf("%v", err)
		return err
	}
	if resp.Unchanged == 1 {
		log.Printf("port doesn't exits\n")
	}
	return nil
}
