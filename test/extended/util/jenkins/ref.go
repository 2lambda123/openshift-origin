package jenkins

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

// JenkinsRef represents a Jenkins instance running on an OpenShift server
type JenkinsRef struct {
	oc   *exutil.CLI
	host string
	port string
	// The namespace in which the Jenkins server is running
	namespace string
	password  string
}

// FlowDefinition can be marshalled into XML to represent a Jenkins workflow job definition.
type FlowDefinition struct {
	XMLName          xml.Name `xml:"flow-definition"`
	Plugin           string   `xml:"plugin,attr"`
	KeepDependencies bool     `xml:"keepDependencies"`
	Definition       Definition
}

// Definition is part of a FlowDefinition
type Definition struct {
	XMLName xml.Name `xml:"definition"`
	Class   string   `xml:"class,attr"`
	Plugin  string   `xml:"plugin,attr"`
	Script  string   `xml:"script"`
}

// ginkgolog creates simple entry in the GinkgoWriter.
func ginkgolog(format string, a ...interface{}) {
	fmt.Fprintf(g.GinkgoWriter, format+"\n", a...)
}

// NewRef creates a jenkins reference from an OC client
func NewRef(oc *exutil.CLI) *JenkinsRef {
	g.By("get ip and port for jenkins service")
	serviceIP, err := oc.Run("get").Args("svc", "jenkins", "--config", exutil.KubeConfigPath()).Template("{{.spec.clusterIP}}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	port, err := oc.Run("get").Args("svc", "jenkins", "--config", exutil.KubeConfigPath()).Template("{{ $x := index .spec.ports 0}}{{$x.port}}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("get admin password")
	password := GetAdminPassword(oc)
	o.Expect(password).ShouldNot(o.BeEmpty())

	j := &JenkinsRef{
		oc:        oc,
		host:      serviceIP,
		port:      port,
		namespace: oc.Namespace(),
		password:  password,
	}
	return j
}

// Namespace returns the Jenkins namespace
func (j *JenkinsRef) Namespace() string {
	return j.namespace
}

// BuildURI builds a URI for the Jenkins server.
func (j *JenkinsRef) BuildURI(resourcePathFormat string, a ...interface{}) string {
	resourcePath := fmt.Sprintf(resourcePathFormat, a...)
	return fmt.Sprintf("http://%s:%v/%s", j.host, j.port, resourcePath)
}

// GetResource submits a GET request to this Jenkins server.
// Returns a response body and status code or an error.
func (j *JenkinsRef) GetResource(resourcePathFormat string, a ...interface{}) (string, int, error) {
	uri := j.BuildURI(resourcePathFormat, a...)
	ginkgolog("Retrieving Jenkins resource: %q", uri)
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return "", 0, fmt.Errorf("Unable to build request for uri %q: %v", uri, err)
	}

	// http://stackoverflow.com/questions/17714494/golang-http-request-results-in-eof-errors-when-making-multiple-requests-successi
	req.Close = true

	req.SetBasicAuth("admin", j.password)
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return "", 0, fmt.Errorf("Unable to GET uri %q: %v", uri, err)
	}

	defer resp.Body.Close()
	status := resp.StatusCode

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("Error reading GET response %q: %v", uri, err)
	}

	return string(body), status, nil
}

// Post sends a POST to the Jenkins server. Returns response body and status code or an error.
func (j *JenkinsRef) Post(reqBody io.Reader, resourcePathFormat, contentType string, a ...interface{}) (string, int, error) {
	uri := j.BuildURI(resourcePathFormat, a...)

	req, err := http.NewRequest("POST", uri, reqBody)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	// http://stackoverflow.com/questions/17714494/golang-http-request-results-in-eof-errors-when-making-multiple-requests-successi
	req.Close = true

	if reqBody != nil {
		req.Header.Set("Content-Type", contentType)
		req.Header.Del("Expect") // jenkins will return 417 if we have an expect hdr
	}
	req.SetBasicAuth("admin", j.password)

	client := &http.Client{}
	ginkgolog("Posting to Jenkins resource: %q", uri)
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("Error posting request to %q: %v", uri, err)
	}

	defer resp.Body.Close()
	status := resp.StatusCode

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("Error reading Post response body %q: %v", uri, err)
	}

	return string(body), status, nil
}

// PostXML sends a POST to the Jenkins server. If a body is specified, it should be XML.
// Returns response body and status code or an error.
func (j *JenkinsRef) PostXML(reqBody io.Reader, resourcePathFormat string, a ...interface{}) (string, int, error) {
	return j.Post(reqBody, resourcePathFormat, "application/xml", a...)
}

// GetResourceWithStatus repeatedly tries to GET a jenkins resource with an acceptable
// HTTP status. Retries for the specified duration.
func (j *JenkinsRef) GetResourceWithStatus(validStatusList []int, timeout time.Duration, resourcePathFormat string, a ...interface{}) (string, int, error) {
	var retBody string
	var retStatus int
	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {
		body, status, err := j.GetResource(resourcePathFormat, a...)
		if err != nil {
			ginkgolog("Error accessing resource: %v", err)
			return false, nil
		}
		var found bool
		for _, s := range validStatusList {
			if status == s {
				found = true
				break
			}
		}
		if !found {
			ginkgolog("Expected http status [%v] during GET by recevied [%v]", validStatusList, status)
			return false, nil
		}
		retBody = body
		retStatus = status
		return true, nil
	})
	if err != nil {
		uri := j.BuildURI(resourcePathFormat, a...)
		return "", retStatus, fmt.Errorf("Error waiting for status %v from resource path %s: %v", validStatusList, uri, err)
	}
	return retBody, retStatus, nil
}

// WaitForContent waits for a particular HTTP status and HTML matching a particular
// pattern to be returned by this Jenkins server. An error will be returned
// if the condition is not matched within the timeout period.
func (j *JenkinsRef) WaitForContent(verificationRegEx string, verificationStatus int, timeout time.Duration, resourcePathFormat string, a ...interface{}) (string, error) {
	var matchingContent = ""
	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {

		content, _, err := j.GetResourceWithStatus([]int{verificationStatus}, timeout, resourcePathFormat, a...)
		if err != nil {
			return false, nil
		}

		if len(verificationRegEx) > 0 {
			re := regexp.MustCompile(verificationRegEx)
			if re.MatchString(content) {
				matchingContent = content
				return true, nil
			} else {
				ginkgolog("Content did not match verification regex %q:\n %v", verificationRegEx, content)
				return false, nil
			}
		} else {
			matchingContent = content
			return true, nil
		}
	})

	if err != nil {
		uri := j.BuildURI(resourcePathFormat, a...)
		return "", fmt.Errorf("Error waiting for status %v and verification regex %q from resource path %s: %v", verificationStatus, verificationRegEx, uri, err)
	} else {
		return matchingContent, nil
	}
}

// CreateItem submits XML to create a named item on the Jenkins server.
func (j *JenkinsRef) CreateItem(name string, itemDefXML string) {
	g.By(fmt.Sprintf("Creating new jenkins item: %s", name))
	_, status, err := j.PostXML(bytes.NewBufferString(itemDefXML), "createItem?name=%s", name)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	o.ExpectWithOffset(1, status).To(o.Equal(200))
}

// GetJobBuildNumber returns the current buildNumber on the named project OR "new" if
// there are no builds against a job yet.
func (j *JenkinsRef) GetJobBuildNumber(name string, timeout time.Duration) (string, error) {
	body, status, err := j.GetResourceWithStatus([]int{200, 404}, timeout, "job/%s/lastBuild/buildNumber", name)
	if err != nil {
		return "", err
	}
	if status != 200 {
		return "new", nil
	}
	return body, nil
}

// StartJob triggers a named Jenkins job. The job can be monitored with the
// returned object.
func (j *JenkinsRef) StartJob(jobName string) *JobMon {
	lastBuildNumber, err := j.GetJobBuildNumber(jobName, time.Minute)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	jmon := &JobMon{
		j:               j,
		lastBuildNumber: lastBuildNumber,
		buildNumber:     "",
		jobName:         jobName,
	}

	ginkgolog("Current timestamp for [%s]: %q", jobName, jmon.lastBuildNumber)
	g.By(fmt.Sprintf("Starting jenkins job: %s", jobName))
	_, status, err := j.PostXML(nil, "job/%s/build?delay=0sec", jobName)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	o.ExpectWithOffset(1, status).To(o.Equal(201))

	return jmon
}

// ReadJenkinsJobUsingVars returns the content of a Jenkins job XML file. Instances of the
// string "PROJECT_NAME" are replaced with the specified namespace.
// Variables named in the vars map will also be replaced with their
// corresponding value.
func (j *JenkinsRef) ReadJenkinsJobUsingVars(filename, namespace string, vars map[string]string) string {
	pre := exutil.FixturePath("testdata", "jenkins-plugin", filename)
	post := exutil.ArtifactPath(filename)

	if vars == nil {
		vars = map[string]string{}
	}
	vars["PROJECT_NAME"] = namespace
	err := exutil.VarSubOnFile(pre, post, vars)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	data, err := ioutil.ReadFile(post)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
	return string(data)
}

// ReadJenkinsJob returns the content of a Jenkins job XML file. Instances of the
// string "PROJECT_NAME" are replaced with the specified namespace.
func (j *JenkinsRef) ReadJenkinsJob(filename, namespace string) string {
	return j.ReadJenkinsJobUsingVars(filename, namespace, nil)
}

// BuildDSLJob returns an XML string defining a Jenkins workflow/pipeline DSL job. Instances of the
// string "PROJECT_NAME" are replaced with the specified namespace.
func (j *JenkinsRef) BuildDSLJob(namespace string, scriptLines ...string) (string, error) {
	script := strings.Join(scriptLines, "\n")
	script = strings.Replace(script, "PROJECT_NAME", namespace, -1)
	fd := FlowDefinition{
		Plugin: "workflow-job@2.7",
		Definition: Definition{
			Class:  "org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition",
			Plugin: "workflow-cps@2.18",
			Script: script,
		},
	}
	output, err := xml.MarshalIndent(fd, "  ", "    ")
	ginkgolog("Formulated DSL Project XML:\n%s\n\n", output)
	return string(output), err
}

// GetJobConsoleLogs returns the console logs of a particular buildNumber.
func (j *JenkinsRef) GetJobConsoleLogs(jobName, buildNumber string) (string, error) {
	return j.WaitForContent("", 200, 10*time.Minute, "job/%s/%s/consoleText", jobName, buildNumber)
}

// GetLastJobConsoleLogs returns the last build associated with a Jenkins job.
func (j *JenkinsRef) GetLastJobConsoleLogs(jobName string) (string, error) {
	return j.GetJobConsoleLogs(jobName, "lastBuild")
}

func GetAdminPassword(oc *exutil.CLI) string {
	envs, err := oc.Run("set").Args("env", "dc/jenkins", "--list").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	kvs := strings.Split(envs, "\n")
	for _, kv := range kvs {
		if strings.HasPrefix(kv, "JENKINS_PASSWORD=") {
			s := strings.Split(kv, "=")
			fmt.Fprintf(g.GinkgoWriter, "\nJenkins admin password %s\n", s[1])
			return s[1]
		}
	}
	return "password"
}

// Finds the pod running Jenkins
func FindJenkinsPod(oc *exutil.CLI) *kapi.Pod {
	pods, err := exutil.GetDeploymentConfigPods(oc, "jenkins")
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	if pods == nil || pods.Items == nil {
		g.Fail("No pods matching jenkins deploymentconfig in namespace " + oc.Namespace())
	}

	o.ExpectWithOffset(1, len(pods.Items)).To(o.Equal(1))
	return &pods.Items[0]
}
