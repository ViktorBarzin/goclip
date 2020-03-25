package main

import (
	"os"

	"github.com/viktorbarzin/goclip/cli"
)

func main() {
	app := cli.GetApp()
	app.Run(os.Args)
}
