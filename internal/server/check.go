package server

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/mroy31/gonetem/internal/link"
	"gopkg.in/yaml.v2"
)

var (
	nameRE     = regexp.MustCompile(`^\w+$`)
	switchRE   = regexp.MustCompile(`^\w{1,10}$`)
	nodeTypeRE = regexp.MustCompile(`^docker\.\w+|ovs$`)
	peerRE     = regexp.MustCompile(`^\w+.[0-9]+$`)
	volumeRE   = regexp.MustCompile(`^[^\0]+:[^\0]+$`)
)

func checkNodeConfig(name string, nConfig NodeConfig, nodes []string) error {
	if isEntryExist(nodes, name) {
		return fmt.Errorf("Node '%s' already exist", name)
	}

	if !nameRE.MatchString(name) {
		return fmt.Errorf("Node: '%s' name field is not valid", name)
	}

	if !nodeTypeRE.MatchString(nConfig.Type) {
		return fmt.Errorf("Node: '%s' type field is not valid", nConfig.Type)
	}

	// more check on ovs node
	if nConfig.Type == "ovs" {
		if !switchRE.MatchString(name) {
			return fmt.Errorf("Switch Node: '%s' name field must have less than 10 caracters", name)
		}

		if nConfig.Mpls || len(nConfig.Vrfs) > 0 {
			return fmt.Errorf("Mpls can not be enable on ovswitch")
		}
	}

	// check vrrp configuration
	if len(nConfig.Vrrps) > 0 {
		if nConfig.Type != "docker.router" {
			return fmt.Errorf("Vrrp can only be enable on docker.router node")
		}

		for _, vrrpCnf := range nConfig.Vrrps {
			_, _, err := net.ParseCIDR(vrrpCnf.Address)
			if err != nil {
				return fmt.Errorf("Vrrp address '%s' is not valid: %s", vrrpCnf.Address, err)
			}
		}
	}

	// check volumes configuration
	for _, vBind := range nConfig.Volumes {
		// only hostPath:containerPath syntax is allowed
		if !volumeRE.MatchString(vBind) {
			return fmt.Errorf("[%s/volumes] volume bind '%s' is not valid", name, vBind)
		}

		// check that host path exists
		split := strings.Split(vBind, ":")
		if _, err := os.Stat(split[0]); os.IsNotExist(err) {
			return fmt.Errorf("[%s/volumes] host path '%s' does not exist", name, split[0])
		}
	}

	return nil
}

func checkBridgeConfig(name string, bConfig BridgeConfig, bridges []string) error {
	if isEntryExist(bridges, name) {
		return fmt.Errorf("Bridge '%s' already exist", name)
	}

	if !nameRE.MatchString(name) {
		return fmt.Errorf("Bridge: '%s' name field is not valid", name)
	}

	ns := link.GetRootNetns()
	defer ns.Close()

	if !link.IsLinkExist(bConfig.Host, ns) {
		return fmt.Errorf("Bridge %s: host interface %s not found", name, bConfig.Host)
	}

	return nil
}

func isEntryExist(nodes []string, node string) bool {
	for _, n := range nodes {
		if node == n {
			return true
		}
	}
	return false
}

func isPeerValid(nodes, peers []string, peer string) error {
	if !peerRE.MatchString(peer) {
		return fmt.Errorf("Link: invalid format for peer '%s' (<node>.<ifIndex> required)", peer)
	}

	for _, p := range peers {
		if peer == p {
			return fmt.Errorf("Link: peer '%s' is already used", peer)
		}
	}

	node := strings.Split(peer, ".")[0]
	if !isEntryExist(nodes, node) {
		return fmt.Errorf("Link: node '%s' not exist", node)
	}

	return nil
}

func CheckTopology(filepath string) (*NetemTopology, []error) {
	var errors []error
	var nodes []string
	var bridges []string
	var peers []string
	var topology NetemTopology

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		errors = append(errors, fmt.Errorf("Unable to read topology file '%s':\n\t%w", filepath, err))
		return nil, errors
	}

	err = yaml.Unmarshal(data, &topology)
	if err != nil {
		errors = append(errors, fmt.Errorf("Unable to parse topology file '%s':\n\t%w", filepath, err))
		return nil, errors
	}

	// check nodes
	for name, nConfig := range topology.Nodes {
		if err := checkNodeConfig(name, nConfig, nodes); err != nil {
			errors = append(errors, err)
		}
		nodes = append(nodes, name)
	}

	// check links
	for _, link := range topology.Links {
		if err := isPeerValid(nodes, peers, link.Peer1); err != nil {
			errors = append(errors, err)
			continue
		}
		if err := isPeerValid(nodes, peers, link.Peer2); err != nil {
			errors = append(errors, err)
			continue
		}

		if link.Peer1 == link.Peer2 {
			errors = append(errors, fmt.Errorf("A link can not have the same peer"))
		}

		peers = append(peers, link.Peer1, link.Peer2)

		// check netem parameters
		if link.Delay < 0 {
			errors = append(errors, fmt.Errorf("Link delay must be >= 0 and specified in ms"))
		}
		if link.Jitter < 0 {
			errors = append(errors, fmt.Errorf("Link jitter must be >= 0 and specified in ms"))
		}
		if link.Loss < 0 {
			errors = append(errors, fmt.Errorf("Link loss must be >= 0 and specified in percent"))
		}
		if link.Jitter > 0 && link.Delay == 0 {
			errors = append(errors, fmt.Errorf("You must set delay with jitter"))
		}
		if link.Loss > 100 {
			errors = append(errors, fmt.Errorf("Link loss must be =< 100 and specified in percent"))
		}

		// check tbf parameters
		if link.Rate < 0 {
			errors = append(errors, fmt.Errorf("Link rate must be >= 0 and specified in kbps"))
		}
		if link.Rate > 0 && link.Delay == 0 {
			errors = append(errors, fmt.Errorf("Delay must be > 0 when Link rate is configured"))
		}
		if link.Buffer < 0.0 {
			errors = append(errors, fmt.Errorf("Link buffer must be >= 0 and specified in BDP scale factor"))
		}
		if link.Buffer > 0.0 && link.Rate == 0 {
			errors = append(errors, fmt.Errorf("Link rate must be > 0 when Link buffer is configured"))
		}
	}

	// check bridges
	for bName, bConfig := range topology.Bridges {
		if err := checkBridgeConfig(bName, bConfig, bridges); err != nil {
			errors = append(errors, err)
		}

		for _, peer := range bConfig.Interfaces {
			if err := isPeerValid(nodes, peers, peer); err != nil {
				errors = append(errors, err)
				continue
			}
			peers = append(peers, peer)
		}

		bridges = append(bridges, bName)
	}

	return &topology, errors
}
