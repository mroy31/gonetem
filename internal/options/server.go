package options

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v2"
)

const (
	VERSION               = "0.1.0"
	DEFAULT_CONFIG_FILE   = "/etc/gonetem/config.yaml"
	INITIAL_SERVER_CONFIG = `
listen: "localhost:10110"
docker:
  images:
    server: mroy31/pynetem-server
    host: mroy31/pynetem-host
    router: mroy31/pynetem-frr
    ovs: mroy31/gonetem-ovs
`
)

type NetemServerConfig struct {
	Listen string
	Docker struct {
		Images struct {
			Server string
			Host   string
			Router string
			Ovs    string
		}
	}
}

var (
	ServerConfig = NetemServerConfig{}
)

func InitServerConfig() {
	err := yaml.Unmarshal([]byte(INITIAL_SERVER_CONFIG), &ServerConfig)
	if err != nil {
		log.Fatalf("Unable to initialize server config: %v", err)
	}
}

func CreateConfigFile(config string) error {
	return ioutil.WriteFile(config, []byte(INITIAL_SERVER_CONFIG), 0644)
}

func ParseConfigFile(config string) error {
	data, err := ioutil.ReadFile(config)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, &ServerConfig)
}
