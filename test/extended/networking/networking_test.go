package networking

import (
	"testing"

	flag "github.com/spf13/pflag"

	_ "github.com/openshift/origin/test/extended/networking"

	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	reportDir = flag.String("report-dir", "", "Path to the directory where the JUnit XML reports should be saved. Default is empty, which doesn't generate these reports.")
)

// init initialize the extended testing suite.
// You can set these environment variables to configure extended tests:
// KUBECONFIG - Path to kubeconfig containing embedded authinfo
func init() {
	exutil.InitTest()
}

func TestExtended(t *testing.T) {
	exutil.ExecuteTest(t, *reportDir)
}
