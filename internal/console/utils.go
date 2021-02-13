package console

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/fatih/color"
)

func CmdErrLogger(pipe io.ReadCloser) {
	reader := bufio.NewReader(pipe)
	for {
		output, err := reader.ReadString('\n')

		if err != nil {
			fmt.Println(color.RedString("%v", err))
			return
		}

		fmt.Print(color.YellowString(string(output)))
	}
}

func EditFile(filename, editor string) error {
	cmd := exec.Command(editor, filename)
	if editor == "vi" || editor == "vim" || editor == "nano" {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
	}

	return cmd.Run()
}
