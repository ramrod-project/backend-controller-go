package dockerservicemanager

import (
	"bytes"
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	client "github.com/docker/docker/client"
	"github.com/ramrod-project/backend-controller-go/rethink"
	r "gopkg.in/gorethink/gorethink.v4"
)

func concatPort(port uint32, proto swarm.PortConfigProtocol) string {
	var stringBuf bytes.Buffer

	stringBuf.WriteString(strconv.FormatUint(uint64(port), 10))
	stringBuf.WriteString("/")
	stringBuf.WriteString(string(proto))

	return stringBuf.String()
}

func serviceToEntry(svc swarm.Service) (map[string]interface{}, error) {
	var (
		res = make(map[string]interface{})
	)

	res["ServiceName"] = svc.Spec.Annotations.Name
	if res["ServiceName"] == "AuxiliaryServices" {
		res["Name"] = "AuxServices"
	} else {
		for _, e := range svc.Spec.TaskTemplate.ContainerSpec.Env {
			split := strings.Split(e, "=")
			if split[0] == "PLUGIN" {
				res["Name"] = split[1]
			}
		}
	}
	res["ServiceID"] = svc.ID
	res["DesiredState"] = string(rethink.DesiredStateNull)
	res["State"] = string(rethink.StateActive)
	res["Interface"] = ""
	res["ExternalPorts"] = make([]string, len(svc.Endpoint.Ports))
	for i, p := range svc.Endpoint.Ports {
		res["ExternalPorts"].([]string)[i] = concatPort(p.PublishedPort, p.Protocol)
	}
	res["InternalPorts"] = make([]string, len(svc.Endpoint.Ports))
	for i, p := range svc.Endpoint.Ports {
		res["InternalPorts"].([]string)[i] = concatPort(p.TargetPort, p.Protocol)
	}
	res["OS"] = string(rethink.PluginOSPosix)
	var null *swarm.Placement
	if svc.Spec.TaskTemplate.Placement != null {
		placement := *svc.Spec.TaskTemplate.Placement
		for _, c := range placement.Constraints {
			split := strings.Split(c, "==")
			if split[0] == "node.labels.os" {
				res["OS"] = split[1]
				break
			}
		}
	}
	res["Environment"] = []string{}
	for _, e := range svc.Spec.TaskTemplate.ContainerSpec.Env {
		split := strings.Split(e, "=")
		pattern := regexp.MustCompile(`PORT|PLUGIN|LOGLEVEL|RETHINK_HOST|STAGE`)
		if !pattern.MatchString(split[0]) {
			res["Environment"] = append(res["Environment"].([]string), e)
		}
	}
	return res, nil
}

// StartupServiceStatus checks current running services
// on boot and ensures that they are properly entered
// in the database
func StartupServiceStatus() error {
	ctx := context.Background()
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	session, err := r.Connect(r.ConnectOpts{
		Address: getRethinkHost(),
	})
	if err != nil {
		return err
	}

	services, err := dockerClient.ServiceList(ctx, types.ServiceListOptions{})
	// Check current services to see if already running
	for _, s := range services {
		// If part of stack, ignore
		if _, ok := s.Spec.Annotations.Labels["com.docker.stack.namespace"]; ok {
			continue
		}
		// Otherwise, check db and add/update as necessary
		cursor, err := r.DB("Controller").Table("Plugins").Filter(map[string]string{"ServiceName": s.Spec.Annotations.Name}).Run(session)
		if err != nil {
			return err
		}
		// Get current ID from db if it exists
		// If exists, update, otherwise create
		var (
			doc       map[string]interface{}
			id        string
			operation r.Term
		)
		if cursor.Next(&doc) {
			// Update entry
			id = doc["id"].(string)
			doc, err = serviceToEntry(s)
			if err != nil {
				return err
			}
			doc["id"] = id
			operation = r.DB("Controller").Table("Plugins").Get(id).Update(doc)
		} else {
			// Create entry
			doc, err = serviceToEntry(s)
			if err != nil {
				return err
			}
			operation = r.DB("Controller").Table("Plugins").Insert(doc)
		}
		_, err = operation.Run(session)
		if err != nil {
			return err
		}
	}
	return nil
}
