package main

import (
	"os"

	"github.com/LyricTian/gin-admin/v10/cmd"
	"github.com/urfave/cli/v2"
)

// Usage: go build -ldflags "-X main.VERSION=x.x.x"
var VERSION = "v10.1.0"

// @title ginadmin
// @version v10.1.0
// @description JD self-operated product price monitoring service based on Gin, GORM and Wire DI.
// @schemes http https
// @basePath /
func main() {
	app := cli.NewApp()
	app.Name = "ginadmin"
	app.Version = VERSION
	app.Usage = "JD self-operated product price monitoring service."
	app.Commands = []*cli.Command{
		cmd.StartCmd(),
		cmd.JDLoginCmd(),
		cmd.StopCmd(),
		cmd.VersionCmd(VERSION),
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}
