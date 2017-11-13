package main

import (
	"math/rand"
	"os"
	"runtime"
	"time"

	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	"k8s.io/kubernetes/pkg/util/logs"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/serviceability"
	"github.com/openshift/origin/pkg/oc/cli"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/spf13/pflag"
	_ "k8s.io/kubernetes/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/autoscaling/install"
	_ "k8s.io/kubernetes/pkg/apis/batch/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"))()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	rand.Seed(time.Now().UTC().UnixNano())
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	// This is one crazy side effect.  Doing this changes the version preferences for serialization.
	kubecmd.NewKubectlCommand(clientcmd.New(pflag.NewFlagSet("", pflag.ContinueOnError)), os.Stdin, os.Stdout, os.Stderr)

	cmd := cli.NewCommandCLI("oc", "oc", os.Stdin, os.Stdout, os.Stderr)
	if cmd.UsageFunc() == nil {
		templates.ActsAsRootCommand(cmd, []string{"options"})
	}
	flagtypes.GLog(cmd.PersistentFlags())
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
