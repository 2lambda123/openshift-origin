package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	execFlags = []cli.Flag{
		cli.StringSliceFlag{
			Name:  "env, e",
			Usage: "Set environment variables",
		},
		cli.BoolFlag{
			Name:  "privileged",
			Usage: "Give the process extended Linux capabilities inside the container.  The default is false",
		},
		cli.BoolFlag{
			Name:  "interactive, i",
			Usage: "Not supported.  All exec commands are interactive by default.",
		},
		cli.BoolFlag{
			Name:  "tty, t",
			Usage: "Allocate a pseudo-TTY. The default is false",
		},
		cli.StringFlag{
			Name:  "user, u",
			Usage: "Sets the username or UID used and optionally the groupname or GID for the specified command",
		},
		LatestFlag,
	}
	execDescription = `
	podman exec

	Run a command in a running container
`

	execCommand = cli.Command{
		Name:                   "exec",
		Usage:                  "Run a process in a running container",
		Description:            execDescription,
		Flags:                  execFlags,
		Action:                 execCmd,
		ArgsUsage:              "CONTAINER-NAME",
		SkipArgReorder:         true,
		UseShortOptionHandling: true,
		OnUsageError:           usageErrorHandler,
	}
)

func execCmd(c *cli.Context) error {
	args := c.Args()
	var ctr *libpod.Container
	var err error
	argStart := 1
	if len(args) < 1 && !c.Bool("latest") {
		return errors.Errorf("you must provide one container name or id")
	}
	if len(args) < 2 && !c.Bool("latest") {
		return errors.Errorf("you must provide a command to exec")
	}
	if c.Bool("latest") {
		argStart = 0
	}
	rootless.SetSkipStorageSetup(true)
	cmd := args[argStart:]
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)

	if c.Bool("latest") {
		ctr, err = runtime.GetLatestContainer()
	} else {
		ctr, err = runtime.LookupContainer(args[0])
	}
	if err != nil {
		return errors.Wrapf(err, "unable to exec into %s", args[0])
	}

	pid, err := ctr.PID()
	if err != nil {
		return err
	}
	became, ret, err := rootless.JoinNS(uint(pid))
	if err != nil {
		return err
	}
	if became {
		os.Exit(ret)
	}

	// ENVIRONMENT VARIABLES
	env := defaultEnvVariables
	for _, e := range c.StringSlice("env") {
		split := strings.SplitN(e, "=", 2)
		if len(split) > 1 {
			env[split[0]] = split[1]
		} else {
			env[split[0]] = ""
		}
	}

	if err := readKVStrings(env, []string{}, c.StringSlice("env")); err != nil {
		return errors.Wrapf(err, "unable to process environment variables")
	}
	envs := []string{}
	for k, v := range env {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	return ctr.Exec(c.Bool("tty"), c.Bool("privileged"), envs, cmd, c.String("user"))
}
