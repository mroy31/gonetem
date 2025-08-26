package options

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"regexp"

	"google.golang.org/grpc/credentials"
	"sigs.k8s.io/yaml"
)

const (
	VERSION               = "0.5.0"
	NETEM_ID              = "ntm"
	SERVER_CONFIG_FILE    = "/etc/gonetem/config.yaml"
	MINIMUM_SERVER_CONFIG = `
listen: "localhost:10110"
tls:
  enabled: false
  ca: ""
  cert: ""
  key: ""
workdir: /tmp
docker:
  timeoutop: 60
  nodes:
    router:
      image: mroy31/gonetem-frr
    host:
      image: mroy31/gonetem-host
    server:
      image: mroy31/gonetem-server
  extraNodes: []
`
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
  nodes:
    router:
      type: router
      image: mroy31/gonetem-frr
      volumes: []
      logOutput: false
      commands:
        console: /usr/bin/vtysh
        shell: /bin/bash
        loadConfig:
        - command: /usr/lib/frr/frrinit.sh start
          checkFiles: []
        saveConfig:
        - command: vtysh -w
          checkFiles: []
      configurationFiles:
      - destSuffix: frr.conf
        source: /etc/frr/frr.conf
        label: FRR
    host:
      type: host
      image: mroy31/gonetem-host
      volumes: []
      logOutput: true
      commands:
        console: /bin/bash
        shell: /bin/bash
        loadConfig:
        - command: network-config.py -l /tmp/custom.net.conf
          checkFiles:
          - /tmp/custom.net.conf
        - command: /bin/bash /gonetem-init.sh
          checkFiles:
          - /gonetem-init.sh
        saveConfig:
        - command: network-config.py -s /tmp/custom.net.conf
          checkFiles: []
      configurationFiles:
      - destSuffix: init.conf
        source: /gonetem-init.sh
        label: Init
      - destSuffix: net.conf
        source: /tmp/custom.net.conf
        label: Network
      - destSuffix: ntp.conf
        source: /etc/ntpsec/ntp.conf
        label: NTP
    server:
      type: server
      image: mroy31/gonetem-server
      volumes: []
      logOutput: true
      commands:
        console: /bin/bash
        shell: /bin/bash
        loadConfig:
        - command: network-config.py -l /tmp/custom.net.conf
          checkFiles:
          - /tmp/custom.net.conf
        - command: /bin/bash /gonetem-init.sh
          checkFiles:
          - /gonetem-init.sh
        saveConfig:
        - command: network-config.py -s /tmp/custom.net.conf
          checkFiles: []
      configurationFiles:
      - destSuffix: init.conf
        source: /gonetem-init.sh
        label: Init
      - destSuffix: net.conf
        source: /tmp/custom.net.conf
        label: Network
      - destSuffix: ntp.conf
        source: /etc/ntpsec/ntp.conf
        label: NTP
      - destSuffix: dhcpd.conf
        source: /etc/dhcp/dhcpd.conf
        label: DHCP
      - destSuffix: tftpd-hpa.default
        source: /etc/default/tftpd-hpa
        label: TFTP
      - destSuffix: isc-relay.default
        source: /etc/default/isc-dhcp-relay
        label: DHCP-RELAY
      - destSuffix: bind.default
        source: /etc/default/named
        label: Bind
      configurationFolders:
      - destSuffix: tftp-data.tgz
        source: /srv/tftp
        label: TFTP-Data
      - destSuffix: bind-etc.tgz
        source: /etc/bind
        label: Bind-Data
  extraNodes: []
  ovsImage: mroy31/gonetem-ovs
`
)

type DockerConfiguration struct {
	DestSuffix string
	Source     string
	Label      string
}

type DockerConfigCommand struct {
	Command    string
	CheckFiles []string
}

type DockerNodeConfig struct {
	Type      string
	Image     string
	Volumes   []string
	LogOutput bool
	Commands  struct {
		Console    string
		Shell      string
		LoadConfig []DockerConfigCommand
		SaveConfig []DockerConfigCommand
	}
	ConfigurationFiles   []DockerConfiguration
	ConfigurationFolders []DockerConfiguration
}

type NetemServerConfig struct {
	Listen  string
	Tls     TLSOptions
	Workdir string
	Docker  struct {
		Timeoutop int
		Nodes     struct {
			Router DockerNodeConfig
			Host   DockerNodeConfig
			Server DockerNodeConfig
		}
		ExtraNodes []DockerNodeConfig
		OvsImage   string
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
	return os.WriteFile(config, []byte(MINIMUM_SERVER_CONFIG), 0644)
}

func ParseServerConfig(config string) error {
	data, err := os.ReadFile(config)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, &ServerConfig)
}

func GetDockerImageId(image string) string {
	hasTagRE := regexp.MustCompile(`^\S+:[\w\.]+$`)

	if !hasTagRE.MatchString(image) {
		image = fmt.Sprintf("%s:%s", image, VERSION)
	}

	return image
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
