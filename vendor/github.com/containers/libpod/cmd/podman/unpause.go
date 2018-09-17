package main

import (
	"fmt"
	"os"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	unpauseDescription = `
   podman unpause

   Unpauses one or more running containers.  The container name or ID can be used.
`
	unpauseCommand = cli.Command{
		Name:         "unpause",
		Usage:        "Unpause the processes in one or more containers",
		Description:  unpauseDescription,
		Action:       unpauseCmd,
		ArgsUsage:    "CONTAINER-NAME [CONTAINER-NAME ...]",
		OnUsageError: usageErrorHandler,
	}
)

func unpauseCmd(c *cli.Context) error {
	if os.Geteuid() != 0 {
		return errors.New("unpause is not supported for rootless containers")
	}

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	args := c.Args()
	if len(args) < 1 {
		return errors.Errorf("you must provide at least one container name or id")
	}

	var lastError error
	for _, arg := range args {
		ctr, err := runtime.LookupContainer(arg)
		if err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "error looking up container %q", arg)
			continue
		}
		if err = ctr.Unpause(); err != nil {
			if lastError != nil {
				fmt.Fprintln(os.Stderr, lastError)
			}
			lastError = errors.Wrapf(err, "failed to unpause container %v", ctr.ID())
		} else {
			fmt.Println(ctr.ID())
		}
	}
	return lastError
}
