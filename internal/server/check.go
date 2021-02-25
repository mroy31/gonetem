package server

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/mroy31/gonetem/internal/link"
	"gopkg.in/yaml.v2"
)

var (
	nameRE     = regexp.MustCompile(`^\w+$`)
	nodeTypeRE = regexp.MustCompile(`^docker\.\w+|ovs$`)
	peerRE     = regexp.MustCompile(`^\w+.[0-9]+$`)
)

func checkNodeConfig(nConfig NodeConfig, nodes []string) error {
	if isEntryExist(nodes, nConfig.Name) {
		return fmt.Errorf("Node '%s' already exist", nConfig.Name)
	}

	if !nameRE.MatchString(nConfig.Name) {
		return fmt.Errorf("Node: '%s' name field is not valid", nConfig.Name)
	}

	if !nodeTypeRE.MatchString(nConfig.Type) {
		return fmt.Errorf("Node: '%s' type field is not valid", nConfig.Type)
	}

	// ovs node do not support MPLS
	if nConfig.Type == "ovs" {
		if nConfig.Mpls || len(nConfig.Vrfs) > 0 {
			return fmt.Errorf("Mpls can not be enable on ovswitch")
		}
	}

	return nil
}

func checkBridgeConfig(bConfig BridgeConfig, bridges []string) error {
	if isEntryExist(bridges, bConfig.Name) {
		return fmt.Errorf("Bridge '%s' already exist", bConfig.Name)
	}

	if !nameRE.MatchString(bConfig.Name) {
		return fmt.Errorf("Bridge: '%s' name field is not valid", bConfig.Name)
	}

	ns := link.GetRootNetns()
	defer ns.Close()

	if !link.IsLinkExist(bConfig.Host, ns) {
		return fmt.Errorf("Bridge %s: host interface %s not found", bConfig.Name, bConfig.Host)
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
	for _, node := range topology.Nodes {
		if err := checkNodeConfig(node, nodes); err != nil {
			errors = append(errors, err)
		}
		nodes = append(nodes, node.Name)
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
	}

	// check bridges
	for _, br := range topology.Bridges {
		if err := checkBridgeConfig(br, bridges); err != nil {
			errors = append(errors, err)
		}

		for _, peer := range br.Interfaces {
			if err := isPeerValid(nodes, peers, peer); err != nil {
				errors = append(errors, err)
				continue
			}
			peers = append(peers, peer)
		}

		bridges = append(bridges, br.Name)
	}

	return &topology, errors
}
