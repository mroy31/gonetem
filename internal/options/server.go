package options

import (
	"log"

	"gopkg.in/yaml.v2"
)

const (
	VERSION               = "0.15.0"
	GLOBAL_CONFIG_FILE    = "/etc/gonetem/config.yaml"
	INITIAL_SERVER_CONFIG = `
listen: /tmp/gonetem.ctl
docker:
  images:
    server: mroy31/pynetem-server
    host: mroy31/pynetem-host
    router: mroy31/pynetem-frr
`
)

type NetemServerConfig struct {
	Listen string
	Docker struct {
		Images struct {
			Server string
			Host   string
			Router string
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

func ParseConfigFile(config string) error {
	return nil
}
