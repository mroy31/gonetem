package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/c-bata/go-prompt"
	"github.com/c-bata/go-prompt/completer"
	"github.com/mroy31/gonetem/internal/console"
	"github.com/mroy31/gonetem/internal/options"
)

type Action int

const (
	List Action = 1 << iota
	Connect
	Open
	Create
	Console
)

var (
	action      Action = 0
	server             = flag.String("server", "unix:////tmp/gonetem.ctl", "Server connection")
	createPrj          = flag.String("create", "", "Specify the path of the new project")
	openPrj            = flag.String("open", "", "Specify the path of the project to open")
	listPrj            = flag.Bool("list", false, "List running projects on the server")
	connectPrj         = flag.Bool("connect", false, "Connect to a running project")
	openConsole        = flag.String("console", "", "Open a console to the specified node")
	disableRun         = flag.Bool("disable-start", false, "Just load the project without start it")
)

func NewPrompt(prjID, prjPath string) {
	c := console.NewPromptCompleter()
	e := console.NewNetemPrompt(*server, prjID, prjPath)

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

func main() {
	flag.Parse()
	// check action
	if *listPrj {
		action |= List
	}
	if *connectPrj {
		action |= Connect
	}
	if *createPrj != "" {
		action |= Create
	}
	if *openPrj != "" {
		action |= Open
	}
	if *openConsole != "" {
		action |= Console
	}

	if action == List {
		if projects := console.ListProjects(*server); projects != nil {
			for _, prj := range projects.GetProjects() {
				fmt.Printf("Id: %s | Name: %s\n", prj.GetId(), prj.GetName())
			}
		}
	} else if action == Connect {
		if projects := console.ListProjects(*server); projects != nil {
			prjID := prompt.Input("Select project: ", console.NewConnectCompleter(projects).Complete)
			NewPrompt(prjID, "")
		}
	} else if action == Open || action == Create {
		prjPath := *openPrj
		if action == Create {
			prjPath = *createPrj
			// first check that project do not exist
			if _, err := os.Stat(prjPath); err == nil {
				console.Fatal("Project %s already exist", prjPath)
			}

			if err := console.CreateProject(prjPath); err != nil {
				console.Fatal("Unable to create project %s: %v", prjPath, err)
			}
		}

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Open project " + filepath.Base(prjPath) + " :"
		s.Start()

		prjID, err := console.OpenProject(*server, prjPath)
		s.Stop()

		if err != nil {
			console.Fatal("Unable to open project: %v", err)
		}

		if !*disableRun {
			s.Prefix = "Start project " + filepath.Base(prjPath) + " :"
			s.Start()

			err := console.StartProject(*server, prjID)
			s.Stop()
			if err != nil {
				console.RedPrintf("Error when starting the project: %v\n", err)
			}
		}

		NewPrompt(prjID, prjPath)
	} else if action == Console {
		if err := console.StartRemoteConsole(*server, *openConsole); err != nil {
			console.Fatal("Console to node %s returns an error: %v", *openConsole, err)
		}
	} else {
		console.Fatal("No action or several actions have been specified ")
	}

}
