package console

import (
	"html/template"
	"os"
	"strconv"

	"github.com/mroy31/gonetem/internal/options"
	"github.com/spf13/cobra"
)

const (
	CONFIG_TPL = `server: {{.Server}}
editor: {{.Editor}}
terminal: {{.Terminal}}\n
tls:
  enabled: {{.Tls.Enabled}}
  ca: {{.Tls.Ca}}
  cert: {{.Tls.Cert}}
  key: {{.Tls.Key}}
`
)

func getConfigCmd() *cobra.Command {
	var configCmd = &cobra.Command{
		Use:   "config",
		Short: "Configure gonetem-console",
		Long:  "Configure gonetem-console, gonetem-config config help for details",
	}

	configCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration of gonetem-console",
		Long:  "Show current configuration of gonetem-console",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			confTpl, err := template.New("config").Parse(CONFIG_TPL)
			if err != nil {
				panic(err)
			}

			if err := confTpl.Execute(os.Stdout, options.ConsoleConfig); err != nil {
				panic(err)
			}
		},
	})

	configCmd.AddCommand(&cobra.Command{
		Use:   "set",
		Short: "Modify a config option",
		Long:  "Modify a config option: gonetem-console config set <key> <value>",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "editor":
				options.ConsoleConfig.Editor = args[1]
			case "server":
				options.ConsoleConfig.Server = args[1]
			case "terminal":
				options.ConsoleConfig.Terminal = args[1]
			case "tls.enabled":
				v, err := strconv.ParseBool(args[1])
				if err != nil {
					Fatal("Unable to parse value '%s' as boolean - %v", args[1], err)
				}
				options.ConsoleConfig.Tls.Enabled = v
			case "tls.ca":
				options.ConsoleConfig.Tls.Ca = args[1]
			case "tls.cert":
				options.ConsoleConfig.Tls.Cert = args[1]
			case "tls.key":
				options.ConsoleConfig.Tls.Key = args[1]
			default:
				Fatal("Unknown config key '%s'", args[0])
			}

			options.SaveConsoleConfig()
		},
	})

	return configCmd
}
