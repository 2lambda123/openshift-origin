package etcd

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	testlibraryapi "github.com/openshift/library-go/test/library/apiserver"
	scalingtestinglibrary "github.com/openshift/origin/test/extended/etcd/helpers"
	machineapihelpers "github.com/openshift/origin/test/extended/machines"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

var _ = g.Describe("[sig-etcd][Serial] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-scaling").AsAdmin()

	// The following test covers a basic vertical scaling scenario.
	// It starts by adding a new master machine to the cluster
	// next it validates the size of etcd cluster and makes sure the new member is healthy.
	// The test ends by removing the newly added machine and validating the size of the cluster
	// and asserting the member was removed from the etcd cluster by contacting MemberList API.
	g.It("is able to vertically scale up and down with a single node", func() {
		// prerequisite, we do run the test only on a platform that supports functional MachineAPI
		// we also currently skip it on metal as it is broken
		dc, err := dynamic.NewForConfig(oc.AdminConfig())
		o.Expect(err).ToNot(o.HaveOccurred())
		machineapihelpers.SkipUnlessMachineAPIOperator(dc, oc.KubeClient().CoreV1().Namespaces())
		skipIfBareMetal(oc)

		// set up
		ctx := context.TODO()
		etcdClientFactory := scalingtestinglibrary.NewEtcdClientFactory(oc.KubeClient())
		machineClientSet, err := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).ToNot(o.HaveOccurred())
		machineClient := machineClientSet.MachineV1beta1().Machines("openshift-machine-api")

		// assert the cluster state before we run the test
		err = scalingtestinglibrary.EnsureInitialClusterState(ctx, g.GinkgoT(), etcdClientFactory, machineClient)
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 1: add a new master node and wait until it is in Running state
		machineName, err := scalingtestinglibrary.CreateNewMasterMachine(ctx, g.GinkgoT(), machineClient)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = scalingtestinglibrary.EnsureMasterMachine(ctx, g.GinkgoT(), machineName, machineClient)
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 2: wait until a new member shows up and check if it is healthy
		//         and until all kube-api servers have reached the same revision
		//         this additional step is the best-effort of ensuring they
		//         have observed the new member before disruption
		err = scalingtestinglibrary.EnsureVotingMembersCount(g.GinkgoT(), etcdClientFactory, 4)
		o.Expect(err).ToNot(o.HaveOccurred())
		memberName, err := scalingtestinglibrary.MachineNameToEtcdMemberName(ctx, oc.KubeClient(), machineClient, machineName)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = scalingtestinglibrary.EnsureHealthyMember(g.GinkgoT(), etcdClientFactory, memberName)
		o.Expect(err).ToNot(o.HaveOccurred())
		g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
		err = testlibraryapi.WaitForAPIServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc.KubeClient().CoreV1().Pods("openshift-kube-apiserver"))
		o.Expect(err).ToNot(o.HaveOccurred())

		// step 3: clean-up: delete the machine and wait until etcd member is removed from the etcd cluster
		defer func() {
			// since the deletion triggers a new rollout
			// we need to make sure that the API is stable after the test
			// so that other e2e test won't hit an API that undergoes a termination (write request might fail)
			g.GinkgoT().Log("waiting for api servers to stabilize on the same revision")
			err = testlibraryapi.WaitForAPIServerToStabilizeOnTheSameRevision(g.GinkgoT(), oc.KubeClient().CoreV1().Pods("openshift-kube-apiserver"))
			o.Expect(err).ToNot(o.HaveOccurred())
		}()
		err = machineClient.Delete(ctx, machineName, metav1.DeleteOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		framework.Logf("successfully deleted the machine %q from the API", machineName)
		err = scalingtestinglibrary.EnsureVotingMembersCount(g.GinkgoT(), etcdClientFactory, 3)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = scalingtestinglibrary.EnsureMemberRemoved(etcdClientFactory, memberName)
		o.Expect(err).ToNot(o.HaveOccurred())
		err = scalingtestinglibrary.EnsureMasterMachinesAndCount(ctx, g.GinkgoT(), machineClient)
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})

func skipIfBareMetal(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if infra.Status.PlatformStatus.Type == configv1.BareMetalPlatformType {
		e2eskipper.Skipf("this test is currently broken on the metal platform and needs to be fixed")
	}
}
