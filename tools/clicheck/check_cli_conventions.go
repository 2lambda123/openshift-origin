package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/pkg/oc/cli"
	"github.com/openshift/origin/tools/clicheck/sanity"
)

func main() {
	oc := cli.NewCommandCLI("oc", "oc", os.Stdin, os.Stdout, os.Stderr)
	errors := sanity.CheckCmdTree(oc, sanity.AllCmdChecks, nil)
	if len(errors) > 0 {
		for i, err := range errors {
			fmt.Fprintf(os.Stderr, "%d. %s\n\n", i+1, err)
		}
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, "Congrats, CLI looks good!")
}
