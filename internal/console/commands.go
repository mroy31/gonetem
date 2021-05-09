package console

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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
links:
bridges:
`
)

var (
	serverFlag     string
	disableRun     bool
	prjRunName     string
	isConsoleShell bool
)

func getServerUri() string {
	if serverFlag != "" {
		return serverFlag
	}
	return options.ConsoleConfig.Server
}

func ListProjects() *proto.PrjListResponse {
	client, err := NewClient(getServerUri())
	if err != nil {
		Fatal("Unable to connect to server identified by uri '%s'\n\t%v", getServerUri(), err)
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

func OpenProject(prjPath string) (string, string, error) {
	data, err := ioutil.ReadFile(prjPath)
	if err != nil {
		return "", "", err
	}

	client, err := NewClient(getServerUri())
	if err != nil {
		return "", "", fmt.Errorf("Unable to connect to server identified by uri '%s'\n\t%v", getServerUri(), err)
	}
	defer client.Conn.Close()

	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "Open project " + filepath.Base(prjPath) + " : "
	s.Start()

	name := prjRunName
	if name == "" {
		// use filename as name
		name = strings.TrimSuffix(filepath.Base(prjPath), ".gnet")
	}
	response, err := client.Client.OpenProject(context.Background(), &proto.OpenRequest{
		Name: name,
		Data: data,
	})
	s.Stop()

	if err != nil {
		return name, "", err
	} else if response.GetStatus().GetCode() == proto.StatusCode_ERROR {
		return name, "", fmt.Errorf(response.GetStatus().GetError())
	}

	prjID := response.GetId()
	if !disableRun {
		s.Prefix = "Start project " + name + " : "
		s.Start()

		_, err := client.Client.Run(context.Background(), &proto.ProjectRequest{Id: prjID})
		s.Stop()

		if err != nil {
			return name, prjID, err
		}
	}

	return name, prjID, nil
}

func NewPrompt(prjName, prjID, prjPath string) {
	e := NewNetemPrompt(getServerUri(), prjID, prjPath)
	c := NewPromptCompleter(e)

	fmt.Println("Welcome to gonetem " + options.VERSION)
	fmt.Println("Please use `exit` to close the project")
	p := prompt.New(
		e.Execute,
		c.Complete,
		prompt.OptionTitle("gonetem-emulator"),
		prompt.OptionPrefix(fmt.Sprintf("[%s]> ", prjName)),
		prompt.OptionCompletionWordSeparator(completer.FilePathCompletionSeparator),
	)
	p.Run()
}

var rootCmd = &cobra.Command{
	Use:   "gonetem-console",
	Short: "gonetem-console is a cli client for gonetem emulator",
	Long:  "gonetem-console is a cli client for gonetem emulator",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of gonetem",
	Long:  `All software has versions. This is gonetem's`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Gonetem network emulator v" + options.VERSION)
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Prune containers not used by any project",
	Long:  "Prune containers not used by any project",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		confirm := prompt.Input("Are you sure you want prune unused containers ? ", ConfirmComplete)
		if confirm == "yes" {
			client, err := NewClient(getServerUri())
			if err != nil {
				Fatal("Unable to connect to server identified by uri '%s'\n\t%v", getServerUri(), err)
			}
			defer client.Conn.Close()

			s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
			s.Prefix = "Clean command launched: "
			s.Start()

			_, err = client.Client.Clean(context.Background(), &emptypb.Empty{})
			s.Stop()

			if err != nil {
				Fatal("Unable to clean old containers: %v", err)
			}
		}
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List running projects on the server",
	Long:  `List running projects on the server`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		projects := ListProjects()

		if len(projects.GetProjects()) == 0 {
			fmt.Println(color.YellowString("No project open on the server"))
		} else {
			for _, prj := range projects.GetProjects() {
				fmt.Printf("Name: %s | OpenAt %s\n", prj.GetName(), prj.GetOpenAt())
			}
		}
	},
}

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a running project",
	Long:  `Connect console to a running project"`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		projects := ListProjects()

		if len(projects.GetProjects()) == 0 {
			fmt.Println(color.YellowString("No project open on the server"))
		} else {
			prjID := ""
			prjName := prompt.Input("Select project: ", NewConnectCompleter(projects).Complete)

			// find project in list
			for _, prj := range projects.GetProjects() {
				if prj.GetName() == prjName {
					prjID = prj.GetId()
				}
			}
			if prjID != "" {
				NewPrompt(prjName, prjID, "")
			}
		}
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a project",
	Long:  `Create a new project, start it and launch console on it"`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if filepath.Ext(args[0]) != ".gnet" {
			Fatal("gonetem accepts only project with .gnet extension")
		}

		if _, err := os.Stat(args[0]); err == nil {
			Fatal("Project %s already exist", args[0])
		}

		if err := CreateProject(args[0]); err != nil {
			Fatal("Unable to create project %s: \n\t%v\n", args[0], err)
		}

		fmt.Println(color.GreenString("Project " + args[0] + " has been created"))
		fmt.Println(color.GreenString("You can now launch it with the command: gonetem-console open " + args[0]))
	},
}

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open a project",
	Long:  `Open a project, start it and launch console on it"`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if filepath.Ext(args[0]) != ".gnet" {
			Fatal("gonetem accepts only project with .gnet extension")
		}

		prjName, prjID, err := OpenProject(args[0])
		if err != nil {
			RedPrintf("Error when open project: \n%v\n", err)
		}

		if prjID != "" {
			NewPrompt(prjName, prjID, args[0])
		}
	},
}

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Open a console to the specified node",
	Long:  `Open a console to the specified node"`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := StartRemoteConsole(getServerUri(), args[0], isConsoleShell); err != nil {
			Fatal("Console to node %s returns an error: %v", args[0], err)
		}
	},
}

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull required docker images on the server",
	Long:  "Pull required docker images on the server",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var s *spinner.Spinner

		client, err := NewClient(getServerUri())
		if err != nil {
			Fatal("Unable to connect to server identified by uri '%s'\n\t%v", getServerUri(), err)
		}
		defer client.Conn.Close()

		stream, err := client.Client.PullImages(context.Background(), &emptypb.Empty{})
		if err != nil {
			Fatal("Unable to pull gonetem images: %v", err)
		}

		for {
			msg, err := stream.Recv()
			if s != nil {
				s.Stop()
			}

			if err == io.EOF {
				break
			} else if err != nil {
				Fatal("Error while pulling gonetem images: %v", err)
			}

			switch msg.Code {
			case proto.PullSrvMsg_START:
				s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
				s.Prefix = "Pull image " + msg.Image + " : "
				s.Start()
			case proto.PullSrvMsg_ERROR:
				fmt.Println(color.RedString(msg.Error))
			case proto.PullSrvMsg_OK:
				fmt.Println(color.GreenString("Image " + msg.Image + " has been pulled"))
			}
		}
	},
}

func Init() {
	rootCmd.PersistentFlags().StringVarP(
		&serverFlag, "server", "s", "",
		"Override server uri defined in config file")
	openCmd.Flags().BoolVar(
		&disableRun, "no-start", false,
		"Do not start the project after open it")
	openCmd.Flags().StringVar(
		&prjRunName, "name", "",
		"Name used to identify the project on the server (name of the file by default)")
	consoleCmd.Flags().BoolVar(
		&isConsoleShell, "shell", false,
		"Open console in shell mode")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(consoleCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(getConfigCmd())
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
