package server

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

func checkNodeConfig(nConfig NodeConfig) error {
	if nConfig.Name == "" {
		return fmt.Errorf("Node: name field is required")
	}
	if nConfig.Type == "" {
		return fmt.Errorf("Node %s: type field is required", nConfig.Name)
	}

	return nil
}

func isNodeExist(nodes []string, node string) bool {
	for _, n := range nodes {
		if node == n {
			return true
		}
	}
	return false
}

func isPeerValid(nodes, peers []string, peer string) error {
	re := regexp.MustCompile(`^\w+.[0-9]+$`)
	if !re.MatchString(peer) {
		return fmt.Errorf("Link: invalid format for peer '%s' (<node>.<ifIndex> required)", peer)
	}

	for _, p := range peers {
		if peer == p {
			return fmt.Errorf("Link: peer '%s' is already used", peer)
		}
	}

	node := strings.Split(peer, ".")[0]
	if !isNodeExist(nodes, node) {
		return fmt.Errorf("Link: node '%s' not exist", node)
	}

	return nil
}

func CheckTopology(filepath string) (*NetemTopology, []error) {
	var errors []error
	var nodes []string
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
		nodes = append(nodes, node.Name)
		if err := checkNodeConfig(node); err != nil {
			errors = append(errors, err)
		}
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

		peers = append(peers, link.Peer1, link.Peer2)
	}

	return &topology, errors
}
