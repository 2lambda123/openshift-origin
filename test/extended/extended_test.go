package extended

import (
	"testing"

	_ "github.com/openshift/origin/test/extended/builds"
	_ "github.com/openshift/origin/test/extended/cli"
	_ "github.com/openshift/origin/test/extended/deployments"
	_ "github.com/openshift/origin/test/extended/dns"
	_ "github.com/openshift/origin/test/extended/images"
	_ "github.com/openshift/origin/test/extended/jenkins"
	_ "github.com/openshift/origin/test/extended/jobs"
	_ "github.com/openshift/origin/test/extended/localquota"
	_ "github.com/openshift/origin/test/extended/networking"
	_ "github.com/openshift/origin/test/extended/router"
	_ "github.com/openshift/origin/test/extended/security"

	exutil "github.com/openshift/origin/test/extended/util"
)

// init initialize the extended testing suite.
func init() {
	exutil.InitTest()
}

func TestExtended(t *testing.T) {
	exutil.ExecuteTest(t, "Extended")
}
