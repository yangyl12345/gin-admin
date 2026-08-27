package cmd

import (
	"context"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/jd"
	"github.com/urfave/cli/v2"
)

func JDLoginCmd() *cli.Command {
	return &cli.Command{
		Name:  "jd-login",
		Usage: "Open an isolated Chrome profile for manual JD authentication",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "workdir", Aliases: []string{"d"}, Value: "configs", Usage: "Working directory"},
			&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Value: "dev", Usage: "Runtime configuration files or directory"},
		},
		Action: func(c *cli.Context) error {
			workDir := c.String("workdir")
			config.MustLoad(workDir, c.String("config"))
			config.C.General.WorkDir = workDir
			return jd.RunLogin(context.Background())
		},
	}
}
