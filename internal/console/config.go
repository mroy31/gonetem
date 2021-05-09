package console

import (
	"html/template"
	"log"
	"os"

	"github.com/mroy31/gonetem/internal/options"
	"github.com/spf13/cobra"
)

const (
	CONFIG_TPL = "Server: {{.Server}}\nEditor: {{.Editor}}\nTerminal: {{.Terminal}}\n"
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
			default:
				log.Fatalf("Unknown config key '%s'", args[0])
			}

			options.SaveConsoleConfig()
		},
	})

	return configCmd
}
