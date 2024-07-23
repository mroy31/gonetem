package options

import (
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"os/user"
	"path"

	"google.golang.org/grpc/credentials"
	"sigs.k8s.io/yaml"
)

const (
	CONSOLE_CONFIG_FILENAME = "console.yaml"
	INITIAL_CONSOLE_CONFIG  = `
server: "localhost:10110"
editor: vim
terminal: "xterm -xrm 'XTerm.vt100.allowTitleOps: false' -title {{.Name}} -e {{.Cmd}}"
tls:
  enabled: false
  ca: ""
  cert: ""
  key: ""
`
)

type NetemConsoleConfig struct {
	Server   string
	Editor   string
	Terminal string
	Tls      TLSOptions
}

var (
	ConsoleConfig = NetemConsoleConfig{}
)

func InitConsoleConfig() {
	err := yaml.Unmarshal([]byte(INITIAL_CONSOLE_CONFIG), &ConsoleConfig)
	if err != nil {
		log.Fatalf("Unable to initialize console config: %v", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf("Unable to get current user: %v", err)
	}

	config := path.Join(currentUser.HomeDir, ".config", "gonetem-console", CONSOLE_CONFIG_FILENAME)
	if err := os.MkdirAll(path.Dir(config), 0755); err != nil {
		log.Fatalf("Unable to create config directory for user: %s", err)
	}

	if _, err := os.Stat(config); err == nil {
		data, err := os.ReadFile(config)
		if err != nil {
			log.Fatalf("Unable to read user config file %s: %s", config, err)
		}
		if err := yaml.Unmarshal(data, &ConsoleConfig); err != nil {
			log.Fatalf("Unable to read user config file %s: %s", config, err)
		}
	} else {
		if err := SaveConsoleConfig(); err != nil {
			log.Fatalf("Unable to save initial user config: %s", err)
		}
	}
}

func SaveConsoleConfig() error {
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("unable to get current user: %f", err)
	}

	config := path.Join(currentUser.HomeDir, ".config", "gonetem-console", CONSOLE_CONFIG_FILENAME)
	data, _ := yaml.Marshal(ConsoleConfig)
	return os.WriteFile(config, data, 0644)
}

func LoadConsoleTLSCredentials() (credentials.TransportCredentials, error) {
	certPool, consoleCerts, err := loadTLSCerts(ConsoleConfig.Tls)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: consoleCerts,
		RootCAs:      certPool,
	}

	return credentials.NewTLS(config), nil
}
