package options

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"

	"gopkg.in/yaml.v2"
)

const (
	CONSOLE_CONFIG_FILENAME = "console.yaml"
	INITIAL_CONSOLE_CONFIG  = `
server: "localhost:10110"
editor: vim
terminal: "xterm -xrm 'XTerm.vt100.allowTitleOps: false' -title {{.Name}} -e {{.Cmd}}"
`
)

type NetemConsoleConfig struct {
	Server   string
	Editor   string
	Terminal string
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
		data, err := ioutil.ReadFile(config)
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
		return fmt.Errorf("Unable to get current user: %f", err)
	}

	config := path.Join(currentUser.HomeDir, ".config", "gonetem-console", CONSOLE_CONFIG_FILENAME)
	data, _ := yaml.Marshal(ConsoleConfig)
	return ioutil.WriteFile(config, data, 0644)
}
