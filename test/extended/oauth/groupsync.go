package oauth

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"github.com/openshift/origin/test/extended/testdata"
	testutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/oauth][Serial] ldap group sync", func() {
	defer g.GinkgoRecover()
	var (
		oc                 = testutil.NewCLI("ldap-group-sync", testutil.KubeConfigPath())
		remoteTmp          = "/tmp/"
		caFileName         = "ca"
		kubeConfigFileName = "kubeconfig"
	)
	g.It("can sync groups from ldap", func() {
		g.By("starting an openldap server")
		ldapService, ca, err := testutil.CreateLDAPTestServer(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("running oc adm groups sync against the ldap server")
		_, err = oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "anyuid", oc.Username()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		pod, err := testutil.NewPodExecutor(oc, "groupsync", "fedora:29")
		o.Expect(err).NotTo(o.HaveOccurred())

		// Install stuff needed for the exec pod to run groupsync.sh and hack/lib
		_, err = pod.Exec("dnf install findutils golang docker which bc openldap-clients -y")
		o.Expect(err).NotTo(o.HaveOccurred())

		// Copy oc
		ocAbsPath, err := exec.LookPath("oc")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = pod.CopyFromHost(ocAbsPath, path.Join("/usr", "bin")+"/")
		o.Expect(err).NotTo(o.HaveOccurred())

		// Copy groupsync test data
		err = pod.CopyFromHost(path.Join("test", "extended", "authentication", "ldap"), remoteTmp)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Copy hack lib needed by groupsync.sh
		err = pod.CopyFromHost("hack", path.Join("/usr", "hack"))
		o.Expect(err).NotTo(o.HaveOccurred())

		// Write ldap CA and kubeconfig to temporary files, and copy them in.
		tmpDir, err := ioutil.TempDir("", "staging")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(tmpDir)

		ldapCAPath := path.Join(tmpDir, caFileName)
		err = ioutil.WriteFile(ldapCAPath, ca, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = pod.CopyFromHost(ldapCAPath, remoteTmp)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = pod.CopyFromHost(testutil.KubeConfigPath(), remoteTmp)
		o.Expect(err).NotTo(o.HaveOccurred())

		groupSyncScriptPath := path.Join(tmpDir, "groupsync.sh")
		groupSyncScript := testdata.MustAsset("test/extended/testdata/ldap/groupsync.sh")
		err = ioutil.WriteFile(groupSyncScriptPath, groupSyncScript, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Copy groupsync script
		err = pod.CopyFromHost(groupSyncScriptPath, path.Join("/usr", "bin", "groupsync.sh"))
		o.Expect(err).NotTo(o.HaveOccurred())

		// Make it executable
		_, err = pod.Exec("chmod +x /usr/bin/groupsync.sh")
		o.Expect(err).NotTo(o.HaveOccurred())

		// Execute groupsync.sh
		_, err = pod.Exec(fmt.Sprintf("export LDAP_SERVICE=%s LDAP_CA=%s ADMIN_KUBECONFIG=%s; groupsync.sh",
			ldapService, path.Join(remoteTmp, caFileName), path.Join(remoteTmp, kubeConfigFileName)))
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
