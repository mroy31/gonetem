package console

import (
	"bufio"
	"fmt"
	"io"

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
