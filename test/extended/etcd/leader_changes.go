package etcd

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	"github.com/openshift/origin/test/extended/prometheus/client"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-etcd] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("etcd-leader-change").AsAdmin()
	g.It("leader changes are not excessive", func() {
		prometheus, err := client.NewE2EPrometheusRouterClient(oc)
		o.Expect(err).ToNot(o.HaveOccurred())
		g.By("Examining the rate of increase in the number of etcd leadership changes for last fifteen minutes")
		result, _, err := prometheus.Query(context.Background(), "round(increase((max by (job) (etcd_server_leader_changes_seen_total) or 0*absent(etcd_server_leader_changes_seen_total))[15m:1s]))", time.Now())
		o.Expect(err).ToNot(o.HaveOccurred())
		leaderChangeLast15Minutes := result.(model.Vector)[0].Value
		if leaderChangeLast15Minutes != 0 {
			o.Expect(fmt.Errorf("Leader changes observed last 15m %q: Leader changes are a result of stopping the etcd leader process or from latency (disk or network), review etcd performance metrics", leaderChangeLast15Minutes)).ToNot(o.HaveOccurred())
		}
	})
})
