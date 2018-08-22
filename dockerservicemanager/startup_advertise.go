package dockerservicemanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	swarm "github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	rethink "github.com/ramrod-project/backend-controller-go/rethink"
	r "gopkg.in/gorethink/gorethink.v4"
)

type ManifestPlugin struct {
	Name string           `json:"Name",omitempty`
	OS   rethink.PluginOS `json:"OS",omitempty`
}

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

func getLeaderHostname() (string, error) {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return "", err
	}

	nodes, err := dockerClient.NodeList(ctx, types.NodeListOptions{})
	if err != nil {
		return "", err
	}

	for _, n := range nodes {
		if n.ManagerStatus.Leader {
			return n.Description.Hostname, nil
		}
	}
	return "", errors.New("no leader found")
}

func getNodes() ([]map[string]interface{}, error) {

	ctx := context.Background()
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
				"Interface":    ip,
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

func getPlugins() ([]ManifestPlugin, error) {
	var plugins []ManifestPlugin

	// Open Manifest file (should be in same directory)
	manifest, err := os.Open("manifest.json")
	if err != nil {
		return nil, err
	}

	jsonParser := json.NewDecoder(manifest)
	if err = jsonParser.Decode(&plugins); err != nil {
		return nil, err
	}

	if len(plugins) < 1 {
		return plugins, fmt.Errorf("no plugins found in manifest.json")
	}

	return plugins, nil
}

func advertisePlugins(manifest []ManifestPlugin) error {
	var plugins []map[string]interface{}

	if len(manifest) < 1 {
		return errors.New("no plugins to advertise")
	}

	session, err := r.Connect(r.ConnectOpts{
		Address: getRethinkHost(),
	})
	if err != nil {
		return err
	}

	for _, plugin := range manifest {
		plugins = append(plugins, map[string]interface{}{
			"Name":          plugin.Name,
			"ServiceID":     "",
			"ServiceName":   "",
			"DesiredState":  "",
			"State":         "Available",
			"Interface":     "",
			"ExternalPorts": []string{},
			"InternalPorts": []string{},
			"OS":            string(plugin.OS),
			"Environment":   []string{},
		})
	}

	_, err = r.DB("Controller").Table("Plugins").Insert(plugins).RunWrite(session)

	return err

}

func advertiseStartupService(service map[string]interface{}) error {
	if service["ServiceName"] == "" {
		return errors.New("service must have ServiceName")
	} else if service["Name"] == "" {
		return errors.New("service must have (plugin) Name")
	}

	session, err := r.Connect(r.ConnectOpts{
		Address: getRethinkHost(),
	})
	if err != nil {
		return err
	}

	_, err = r.DB("Controller").Table("Plugins").Insert(service).RunWrite(session)
	if err != nil {
		return err
	}

	leader, err := getLeaderHostname()
	if err != nil {
		return err
	}

	if len(service["ExternalPorts"].([]string)) > 0 {
		var (
			doc    = make(map[string]interface{})
			filter = map[string]string{
				"NodeHostName": leader,
			}
			newTCP []string
			newUDP []string
		)

		res, err := r.DB("Controller").Table("Ports").Filter(filter).Run(session)
		if err != nil {
			return err
		}

		if res.Next(&doc) {
			for _, tcpPort := range doc["TCPPorts"].([]interface{}) {
				newTCP = append(newTCP, tcpPort.(string))
			}
			for _, udpPort := range doc["UDPPorts"].([]interface{}) {
				newUDP = append(newUDP, udpPort.(string))
			}

			for _, port := range service["ExternalPorts"].([]string) {
				split := strings.Split(port, "/")
				if split[1] == "tcp" {
					newTCP = append(newTCP, split[0])
				} else if split[1] == "udp" {
					newUDP = append(newUDP, split[0])
				} else {
					return fmt.Errorf("port %v not set to tcp or udp", port)
				}
			}

			doc["TCPPorts"] = newTCP
			doc["UDPPorts"] = newUDP

			_, err := r.DB("Controller").Table("Ports").Get(doc["id"]).Update(doc).RunWrite(session)
			if err != nil {
				return err
			}
		} else {
			return errors.New("leader port entry not found")
		}

	}
	return nil
}

func checkForNode(name string) string {
	var doc map[string]interface{}

	session, err := r.Connect(r.ConnectOpts{
		Address: getRethinkHost(),
	})
	if err != nil {
		return ""
	}

	cursor, err := r.DB("Controller").Table("Ports").Run(session)
	for cursor.Next(&doc) {
		if doc["NodeHostName"].(string) == name {
			return doc["id"].(string)
		}
	}
	return ""
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

	for _, e := range entries {
		if resID := checkForNode(e["NodeHostName"].(string)); resID == "" {
			_, err = r.DB("Controller").Table("Ports").Insert(e).RunWrite(session)
		} else {
			_, err = r.DB("Controller").Table("Ports").Get(resID).Update(e).RunWrite(session)
		}
	}

	return err
}

// NodeAdvertise attempts to get the information
// needed from the nodes in the swarm and advertise
// it to the Controller.Ports database.
func NodeAdvertise() error {

	// Read the current info from the nodes
	// in the swarm and create entries
	nodes, err := getNodes()
	if err != nil {
		return err
	}

	// Populate database with node entries
	err = advertiseIPs(nodes)
	return err
}

// PluginAdvertise reads manifest.json and
// populates the database with proper plugin entries
func PluginAdvertise() error {

	// Read the manifest
	manifest, err := getPlugins()
	if err != nil {
		return err
	}

	// Populate the database with the manifest
	// plugins
	err = advertisePlugins(manifest)
	return err
}
