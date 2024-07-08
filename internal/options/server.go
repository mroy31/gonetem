package options

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"regexp"

	"google.golang.org/grpc/credentials"
	"gopkg.in/yaml.v2"
)

const (
	VERSION               = "0.3.0"
	IMG_VERSION           = "0.2.0"
	NETEM_ID              = "ntm"
	SERVER_CONFIG_FILE    = "/etc/gonetem/config.yaml"
	INITIAL_SERVER_CONFIG = `
listen: "localhost:10110"
tls:
  enabled: false
  ca: ""
  cert: ""
  key: ""
workdir: /tmp
docker:
  timeoutop: 60
  images:
    server: mroy31/gonetem-server
    host: mroy31/gonetem-host
    router: mroy31/gonetem-frr
    ovs: mroy31/gonetem-ovs
`
)

type DockerImageT int

const (
	IMG_ROUTER DockerImageT = iota
	IMG_HOST
	IMG_SERVER
	IMG_OVS
)

type NetemServerConfig struct {
	Listen  string
	Tls     TLSOptions
	Workdir string
	Docker  struct {
		Timeoutop int
		Images    struct {
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

func CreateServerConfig(config string) error {
	return os.WriteFile(config, []byte(INITIAL_SERVER_CONFIG), 0644)
}

func ParseServerConfig(config string) error {
	data, err := os.ReadFile(config)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, &ServerConfig)
}

func GetDockerImageId(imgType DockerImageT) string {
	var name string
	hasTagRE := regexp.MustCompile(`^\S+:[\w\.]+$`)

	switch imgType {
	case IMG_ROUTER:
		name = ServerConfig.Docker.Images.Router
	case IMG_HOST:
		name = ServerConfig.Docker.Images.Host
	case IMG_SERVER:
		name = ServerConfig.Docker.Images.Server
	case IMG_OVS:
		name = ServerConfig.Docker.Images.Ovs
	}

	if !hasTagRE.MatchString(name) {
		name = fmt.Sprintf("%s:%s", name, IMG_VERSION)
	}

	return name
}

func LoadServerTLSCredentials() (credentials.TransportCredentials, error) {
	certPool, serverCerts, err := loadTLSCerts(ServerConfig.Tls)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: serverCerts,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	return credentials.NewTLS(config), nil
}
