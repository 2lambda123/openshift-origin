package servingcertsigner

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/util/logs"

	"github.com/golang/glog"
	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	servicecertsignerv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/servingcert"
	"github.com/openshift/service-serving-cert-signer/pkg/version"
)

var (
	componentName = "openshift-service-serving-cert-signer"
	configScheme  = runtime.NewScheme()
)

func init() {
	if err := operatorv1alpha1.AddToScheme(configScheme); err != nil {
		panic(err)
	}
	if err := servicecertsignerv1alpha1.AddToScheme(configScheme); err != nil {
		panic(err)
	}
}

type ControllerCommandOptions struct {
	basicFlags *controllercmd.ControllerFlags
}

func NewController() *cobra.Command {
	o := &ControllerCommandOptions{
		basicFlags: controllercmd.NewControllerFlags(),
	}

	cmd := &cobra.Command{
		Use:   "serving-cert-signer",
		Short: "Start the Service Serving Cert Signer controller",
		Run: func(cmd *cobra.Command, args []string) {
			// boiler plate for the "normal" command
			rand.Seed(time.Now().UTC().UnixNano())
			logs.InitLogs()
			defer logs.FlushLogs()
			defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
			defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()
			serviceability.StartProfiler()

			if err := o.basicFlags.Validate(); err != nil {
				glog.Fatal(err)
			}

			if err := o.StartController(); err != nil {
				glog.Fatal(err)
			}
		},
	}
	o.basicFlags.AddFlags(cmd)

	return cmd
}

// StartController runs the controller
func (o *ControllerCommandOptions) StartController() error {
	uncastConfig, err := o.basicFlags.ToConfigObj(configScheme, servicecertsignerv1alpha1.SchemeGroupVersion)
	if err != nil {
		return err
	}
	// TODO this and how you get the leader election and serving info are the only unique things here
	config, ok := uncastConfig.(*servicecertsignerv1alpha1.ServiceServingCertSignerConfig)
	if !ok {
		return fmt.Errorf("unexpected config: %T", uncastConfig)
	}

	return controllercmd.NewController(componentName, (&servingcert.ServingCertOptions{Config: config}).RunServingCert).
		WithKubeConfigFile(o.basicFlags.KubeConfigFile, nil).
		// TODO we can update the API with leader election
		//WithLeaderElection(config.LeaderElection, "", componentName+"-lock").
		Run()
}
