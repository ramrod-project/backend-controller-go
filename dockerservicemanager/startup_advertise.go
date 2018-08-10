package dockerservicemanager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	rethink "github.com/ramrod-project/backend-controller-go/rethink"
	r "gopkg.in/gorethink/gorethink.v4"
)

var osMap = map[string]rethink.PluginOS{
	"linux":   rethink.PluginOSPosix,
	"windows": rethink.PluginOSWindows,
}

func getRethinkHost() string {
	temp := os.Getenv("STAGE")
	if temp == "TESTING" {
		return "127.0.0.1"
	}
	return "rethinkdb"
}

func getNodes() ([]map[string]interface{}, error) {

	ctx := context.TODO()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	nodes, err := dockerClient.NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		return nil, err
	}

	entries := make([]map[string]interface{}, len(nodes))

	for i, n := range nodes {
		hostname := n.Description.Hostname
		ip := n.Status.Addr
		spec := n.Spec
		if osname, ok := osMap[n.Description.Platform.OS]; ok {
			entry := map[string]interface{}{
				"Address":      ip,
				"NodeHostName": hostname,
				"OS":           string(osname),
				"TCPPorts":     []string{},
				"UDPPorts":     []string{},
			}
			entries[i] = entry
			spec.Annotations.Labels = map[string]string{
				"os": string(osname),
				"ip": ip,
			}
			start := time.Now()
			for time.Since(start) < 5*time.Second {
				inspectNew, _, _ := dockerClient.NodeInspectWithRaw(ctx, n.ID)
				err = dockerClient.NodeUpdate(ctx, n.ID, swarm.Version{Index: inspectNew.Meta.Version.Index}, spec)
				if err == nil {
					break
				}
			}
			if err != nil {
				log.Printf("%v", err)
				return nil, fmt.Errorf("could not assign label to node %v", hostname)
			}
		} else {
			return nil, fmt.Errorf("OS not recognized for node %v", hostname)
		}
	}

	return entries, nil
}

func advertiseIPs(entries []map[string]interface{}) error {

	if len(entries) < 1 {
		return errors.New("no nodes to advertise")
	}

	session, err := r.Connect(r.ConnectOpts{
		Address: getRethinkHost(),
	})
	if err != nil {
		return err
	}

	_, err = r.DB("Controller").Table("Ports").Insert(entries).Run(session)

	return err
}

// NodeAdvertise attempts to get the information
// needed from the nodes in the swarm and advertise
// it to the Controller.Ports database.
func NodeAdvertise() error {

	nodes, err := getNodes()
	if err != nil {
		return err
	}

	err = advertiseIPs(nodes)
	return err
}