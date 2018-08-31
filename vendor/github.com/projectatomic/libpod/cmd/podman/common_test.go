package main

import (
	"os/user"
	"testing"

	"flag"

	"github.com/urfave/cli"
)

func TestGetStore(t *testing.T) {
	t.Skip("FIX THIS!")

	//cmd/podman/common_test.go:27: cannot use c (type *cli.Context) as type *libkpod.Config in argument to getStore

	// Make sure the tests are running as root
	skipTestIfNotRoot(t)

	set := flag.NewFlagSet("test", 0)
	globalSet := flag.NewFlagSet("test", 0)
	globalSet.String("root", "", "path to the root directory in which data, including images,  is stored")
	globalCtx := cli.NewContext(nil, globalSet, nil)
	command := cli.Command{Name: "imagesCommand"}
	c := cli.NewContext(nil, set, globalCtx)
	c.Command = command

	//_, err := getStore(c)
	//if err != nil {
	//t.Error(err)
	//}
}

func skipTestIfNotRoot(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip("Could not determine user.  Running without root may cause tests to fail")
	} else if u.Uid != "0" {
		t.Skip("tests will fail unless run as root")
	}
}
