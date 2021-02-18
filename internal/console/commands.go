package console

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/fatih/color"
	"github.com/mroy31/gonetem/internal/options"
	"github.com/mroy31/gonetem/internal/proto"
	"github.com/mroy31/gonetem/internal/utils"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	networkFilename = "network.yml"
	emptyNetwork    = `
nodes:
`
)

var (
	server     string
	disableRun bool
)

func ListProjects() *proto.PrjListResponse {
	client, err := NewClient(server)
	if err != nil {
		Fatal("Unable to connect to server: %v", err)
	}
	defer client.Conn.Close()

	projects, err := client.Client.GetProjects(context.Background(), &emptypb.Empty{})
	if err != nil {
		Fatal("Unable to get list of projects: %v", err)
	}

	return projects
}

func CreateProject(prjPath string) error {
	prj, err := os.Create(prjPath)
	if err != nil {
		return err
	}
	defer prj.Close()

	return utils.CreateOneFileArchive(prj, networkFilename, []byte(emptyNetwork))
}

func OpenProject(prjPath string) (string, error) {
	data, err := ioutil.ReadFile(prjPath)
	if err != nil {
		return "", err
	}

	client, err := NewClient(server)
	if err != nil {
		return "", fmt.Errorf("Server not responding")
	}
	defer client.Conn.Close()

	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "Open project " + filepath.Base(prjPath) + " :"
	s.Start()

	response, err := client.Client.OpenProject(context.Background(), &proto.OpenRequest{
		Name: filepath.Base(prjPath),
		Data: data,
	})
	s.Stop()

	if err != nil {
		return "", err
	}

	prjID := response.GetId()
	if !disableRun {
		s.Prefix = "Start project " + filepath.Base(prjPath) + " :"
		s.Start()

		_, err := client.Client.Run(context.Background(), &proto.ProjectRequest{Id: prjID})
		s.Stop()

		if err != nil {
			return prjID, err
		}
	}

	return prjID, nil
}

func NewPrompt(prjID, prjPath string) {
	c := NewPromptCompleter()
	e := NewNetemPrompt(server, prjID, prjPath)

	fmt.Println("Welcome to gonetem " + options.VERSION)
	fmt.Println("Please use `exit` or `Ctrl-D` to exit this program.")
	p := prompt.New(
		e.Execute,
		c.Complete,
		prompt.OptionTitle("gonetem-emulator"),
		prompt.OptionPrefix(fmt.Sprintf("[%s]> ", prjID)),
		prompt.OptionInputTextColor(prompt.Yellow),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator),
	)
	p.Run()
}

var rootCmd = &cobra.Command{
	Use:   "gonetem-console",
	Short: "gonetem-console is a console to work with gonetem network emulator",
	Long:  `gonetem-console is a console to work with gonetem network emulator`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of gonetem",
	Long:  `All software has versions. This is gonetem's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Gonetem network emulator v" + options.VERSION)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List running projects on the server",
	Long:  `List running projects on the server`,
	Run: func(cmd *cobra.Command, args []string) {
		projects := ListProjects()

		if len(projects.GetProjects()) == 0 {
			fmt.Println(color.YellowString("No project open on the server"))
		} else {
			for _, prj := range projects.GetProjects() {
				fmt.Printf("Id: %s | Name: %s\n", prj.GetId(), prj.GetName())
			}
		}
	},
}

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a project",
	Long:  `Connect console to a running project"`,
	Run: func(cmd *cobra.Command, args []string) {
		projects := ListProjects()

		if len(projects.GetProjects()) == 0 {
			fmt.Println(color.YellowString("No project open on the server"))
		} else {
			prjID := prompt.Input("Select project: ", NewConnectCompleter(projects).Complete)
			if prjID != "" {
				NewPrompt(prjID, "")
			}
		}
	},
}

var createCmd = &cobra.Command{
	Use:       "create",
	Short:     "Create a project",
	Long:      `Create a new project, start it and launch console on it"`,
	ValidArgs: []string{"project-path"},
	Args:      cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if _, err := os.Stat(args[0]); err == nil {
			Fatal("Project %s already exist", args[0])
		}

		if err := CreateProject(args[0]); err != nil {
			Fatal("Unable to create project %s: %v", args[0], err)
		}

		prjID, err := OpenProject(args[0])
		if err != nil {
			RedPrintf("Error when starting project: %v", err)
		}
		if prjID != "" {
			NewPrompt(prjID, args[0])
		}
	},
}

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open a project",
	Long:  `Open a project, start it and launch console on it"`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		prjID, err := OpenProject(args[0])
		if err != nil {
			RedPrintf("Error when starting project: %v", err)
		}
		if prjID != "" {
			NewPrompt(prjID, args[0])
		}
	},
}

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Open a console to the specified node",
	Long:  `Open a console to the specified node"`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := StartRemoteConsole(server, args[0]); err != nil {
			Fatal("Console to node %s returns an error: %v", args[0], err)
		}
	},
}

func Init() {
	rootCmd.PersistentFlags().StringVarP(&server, "server", "s", "localhost:10110", "Server uri for connection")
	createCmd.Flags().BoolVar(&disableRun, "disable-start", false, "Create a project without start it")
	openCmd.Flags().BoolVar(&disableRun, "disable-start", false, "Create a project without start it")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(consoleCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
