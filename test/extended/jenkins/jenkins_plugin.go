package jenkins

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

func immediateInteractionWithJenkins(uri, method string, body io.Reader, status int) {
	req, err := http.NewRequest(method, uri, body)
	o.Expect(err).NotTo(o.HaveOccurred())

	if body != nil {
		req.Header.Set("Content-Type", "application/xml")
		// jenkins will return 417 if we have an expect hdr
		req.Header.Del("Expect")
	}
	req.SetBasicAuth("admin", "password")

	client := &http.Client{}
	resp, err := client.Do(req)
	o.Expect(err).NotTo(o.HaveOccurred())

	defer resp.Body.Close()
	o.Expect(resp.StatusCode).To(o.BeEquivalentTo(status))

}

func waitForJenkinsActivity(uri, verificationString string, status int) error {
	consoleLogs := ""

	err := wait.Poll(1*time.Second, 3*time.Minute, func() (bool, error) {
		req, err := http.NewRequest("GET", uri, nil)
		if err != nil {
			return false, err
		}
		req.SetBasicAuth("admin", "password")
		client := &http.Client{}
		resp, _ := client.Do(req)
		// the http req failing here (which we see occassionally in the ci.jenkins runs) could stem
		// from simply hitting our test jenkins server too soon ... so rather than returning false,err
		// and aborting the poll, we return false, nil to try again
		if resp == nil {
			return false, nil
		}
		defer resp.Body.Close()
		if resp.StatusCode == status {
			if len(verificationString) > 0 {
				contents, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					return false, err
				}
				consoleLogs = string(contents)
				if strings.Contains(consoleLogs, verificationString) {
					return true, nil
				} else {
					return false, nil
				}
			} else {
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		return fmt.Errorf("got error %v waiting on uri %s with verificationString %s and last set of console logs %s", err, uri, verificationString, consoleLogs)
	} else {
		return nil
	}
}

func jenkinsJobBytes(filename, namespace string) []byte {
	pre := exutil.FixturePath("fixtures", filename)
	post := exutil.ArtifactPath(filename)
	err := exutil.VarSubOnFile(pre, post, "PROJECT_NAME", namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
	data, err := ioutil.ReadFile(post)
	o.Expect(err).NotTo(o.HaveOccurred())
	return data
}

var _ = g.Describe("jenkins: plugin: run job leveraging openshift pipeline plugin", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("jenkins-plugin", exutil.KubeConfigPath())
	var hostPort string

	g.BeforeEach(func() {

		g.By("set up policy for jenkins jobs")
		err := oc.Run("policy").Args("add-role-to-user", "edit", "system:serviceaccount:"+oc.Namespace()+":default").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("kick off the build for the jenkins ephermeral and application templates")
		jenkinsEphemeralPath := exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json")
		err = oc.Run("new-app").Args(jenkinsEphemeralPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		jenkinsApplicationPath := exutil.FixturePath("..", "..", "examples", "jenkins", "application-template.json")
		err = oc.Run("new-app").Args(jenkinsApplicationPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for jenkins deployment")
		err = exutil.WaitForADeployment(oc.KubeREST().ReplicationControllers(oc.Namespace()), "jenkins", exutil.CheckDeploymentCompletedFn, exutil.CheckDeploymentFailedFn)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("get ip and port for jenkins service")
		serviceIP, err := oc.Run("get").Args("svc", "jenkins", "--config", exutil.KubeConfigPath()).Template("{{.spec.clusterIP}}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		port, err := oc.Run("get").Args("svc", "jenkins", "--config", exutil.KubeConfigPath()).Template("{{ $x := index .spec.ports 0}}{{$x.port}}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		hostPort = fmt.Sprintf("%s:%s", serviceIP, port)

		g.By("wait for jenkins to come up")
		err = waitForJenkinsActivity(fmt.Sprintf("http://%s", hostPort), "", 200)
		o.Expect(err).NotTo(o.HaveOccurred())

	})

	g.Context("jenkins-plugin test context  ", func() {

		g.It("jenkins-plugin test case execution", func() {

			g.By("create jenkins job config xml file, convert to bytes for http post")
			data := jenkinsJobBytes("testjob-plugin.xml", oc.Namespace())

			g.By("make http request to create job")
			immediateInteractionWithJenkins(fmt.Sprintf("http://%s/createItem?name=test-plugin-job", hostPort), "POST", bytes.NewBuffer(data), 200)

			g.By("make http request to kick off build")
			immediateInteractionWithJenkins(fmt.Sprintf("http://%s/job/test-plugin-job/build?delay=0sec", hostPort), "POST", nil, 201)

			// the build and deployment is by far the most time consuming portion of the test jenkins job;
			// we leverage some of the openshift utilities for waiting for the deployment before we poll
			// jenkins for the sucessful job completion
			g.By("waiting for frontend, frontend-prod deployments as signs that the build has finished")
			err := exutil.WaitForADeployment(oc.KubeREST().ReplicationControllers(oc.Namespace()), "frontend", exutil.CheckDeploymentCompletedFn, exutil.CheckDeploymentFailedFn)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitForADeployment(oc.KubeREST().ReplicationControllers(oc.Namespace()), "frontend-prod", exutil.CheckDeploymentCompletedFn, exutil.CheckDeploymentFailedFn)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("get build console logs and see if succeeded")
			err = waitForJenkinsActivity(fmt.Sprintf("http://%s/job/test-plugin-job/1/console", hostPort), "SUCCESS", 200)
			o.Expect(err).NotTo(o.HaveOccurred())

		})

	})

})
