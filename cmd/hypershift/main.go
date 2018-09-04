package main

import (
	goflag "flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	utilflag "k8s.io/apiserver/pkg/util/flag"
	"k8s.io/apiserver/pkg/util/logs"

	"github.com/containers/storage/pkg/reexec"

	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver"
	"github.com/openshift/origin/pkg/cmd/openshift-controller-manager"
	"github.com/openshift/origin/pkg/cmd/openshift-etcd"
	"github.com/openshift/origin/pkg/cmd/openshift-experimental"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver"
	"github.com/openshift/origin/pkg/version"
)

func main() {
	if reexec.Init() {
		return
	}

	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	command := NewHyperShiftCommand()
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func NewHyperShiftCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hypershift",
		Short: "Combined server command for OpenShift",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	startEtcd, _ := openshift_etcd.NewCommandStartEtcdServer(openshift_etcd.RecommendedStartEtcdServerName, "hypershift", os.Stdout, os.Stderr)
	startEtcd.Deprecated = "will be removed in 3.10"
	startEtcd.Hidden = true
	cmd.AddCommand(startEtcd)

	startOpenShiftAPIServer := openshift_apiserver.NewOpenShiftAPIServerCommand(openshift_apiserver.RecommendedStartAPIServerName, "hypershift", os.Stdout, os.Stderr)
	cmd.AddCommand(startOpenShiftAPIServer)

	startOpenShiftKubeAPIServer := openshift_kube_apiserver.NewOpenShiftKubeAPIServerServerCommand(openshift_kube_apiserver.RecommendedStartAPIServerName, "hypershift", os.Stdout, os.Stderr)
	cmd.AddCommand(startOpenShiftKubeAPIServer)

	startOpenShiftControllerManager := openshift_controller_manager.NewOpenShiftControllerManagerCommand(openshift_controller_manager.RecommendedStartControllerManagerName, "hypershift", os.Stdout, os.Stderr)
	cmd.AddCommand(startOpenShiftControllerManager)

	experimental := openshift_experimental.NewExperimentalCommand(os.Stdout, os.Stderr)
	cmd.AddCommand(experimental)

	return cmd
}
