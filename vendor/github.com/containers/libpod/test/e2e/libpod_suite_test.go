package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/containers/storage/pkg/parsers/kernel"
	"github.com/containers/storage/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	PODMAN_BINARY      string
	CONMON_BINARY      string
	CNI_CONFIG_DIR     string
	RUNC_BINARY        string
	INTEGRATION_ROOT   string
	CGROUP_MANAGER     = "systemd"
	STORAGE_OPTIONS    = "--storage-driver vfs"
	ARTIFACT_DIR       = "/tmp/.artifacts"
	CACHE_IMAGES       = []string{ALPINE, BB, fedoraMinimal, nginx, redis, registry, infra}
	RESTORE_IMAGES     = []string{ALPINE, BB}
	ALPINE             = "docker.io/library/alpine:latest"
	BB                 = "docker.io/library/busybox:latest"
	BB_GLIBC           = "docker.io/library/busybox:glibc"
	fedoraMinimal      = "registry.fedoraproject.org/fedora-minimal:latest"
	nginx              = "quay.io/baude/alpine_nginx:latest"
	redis              = "docker.io/library/redis:alpine"
	registry           = "docker.io/library/registry:2"
	infra              = "k8s.gcr.io/pause:3.1"
	defaultWaitTimeout = 90
)

// PodmanSession wrapps the gexec.session so we can extend it
type PodmanSession struct {
	*gexec.Session
}

// PodmanTest struct for command line options
type PodmanTest struct {
	PodmanBinary        string
	ConmonBinary        string
	CrioRoot            string
	CNIConfigDir        string
	RunCBinary          string
	RunRoot             string
	StorageOptions      string
	SignaturePolicyPath string
	ArtifactPath        string
	TempDir             string
	CgroupManager       string
}

// HostOS is a simple struct for the test os
type HostOS struct {
	Distribution string
	Version      string
}

// TestLibpod ginkgo master function
func TestLibpod(t *testing.T) {
	if reexec.Init() {
		os.Exit(1)
	}
	if os.Getenv("NOCACHE") == "1" {
		CACHE_IMAGES = []string{}
		RESTORE_IMAGES = []string{}
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Libpod Suite")
}

var _ = BeforeSuite(func() {
	//Cache images
	cwd, _ := os.Getwd()
	INTEGRATION_ROOT = filepath.Join(cwd, "../../")
	podman := PodmanCreate("/tmp")
	podman.ArtifactPath = ARTIFACT_DIR
	if _, err := os.Stat(ARTIFACT_DIR); os.IsNotExist(err) {
		if err = os.Mkdir(ARTIFACT_DIR, 0777); err != nil {
			fmt.Printf("%q\n", err)
			os.Exit(1)
		}
	}
	for _, image := range CACHE_IMAGES {
		if err := podman.CreateArtifact(image); err != nil {
			fmt.Printf("%q\n", err)
			os.Exit(1)
		}
	}
	host := GetHostDistributionInfo()
	if host.Distribution == "rhel" && strings.HasPrefix(host.Version, "7") {
		f, err := os.OpenFile("/proc/sys/user/max_user_namespaces", os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("Unable to enable userspace on RHEL 7")
			os.Exit(1)
		}
		_, err = f.WriteString("15000")
		if err != nil {
			fmt.Println("Unable to enable userspace on RHEL 7")
			os.Exit(1)
		}
		f.Close()
	}
})

// CreateTempDirin
func CreateTempDirInTempDir() (string, error) {
	return ioutil.TempDir("", "podman_test")
}

// PodmanCreate creates a PodmanTest instance for the tests
func PodmanCreate(tempDir string) PodmanTest {

	cwd, _ := os.Getwd()

	podmanBinary := filepath.Join(cwd, "../../bin/podman")
	if os.Getenv("PODMAN_BINARY") != "" {
		podmanBinary = os.Getenv("PODMAN_BINARY")
	}
	conmonBinary := filepath.Join("/usr/libexec/podman/conmon")
	altConmonBinary := "/usr/libexec/podman/conmon"
	if _, err := os.Stat(altConmonBinary); err == nil {
		conmonBinary = altConmonBinary
	}
	if os.Getenv("CONMON_BINARY") != "" {
		conmonBinary = os.Getenv("CONMON_BINARY")
	}
	storageOptions := STORAGE_OPTIONS
	if os.Getenv("STORAGE_OPTIONS") != "" {
		storageOptions = os.Getenv("STORAGE_OPTIONS")
	}
	cgroupManager := CGROUP_MANAGER
	if os.Getenv("CGROUP_MANAGER") != "" {
		cgroupManager = os.Getenv("CGROUP_MANAGER")
	}

	runCBinary := "/usr/bin/runc"
	CNIConfigDir := "/etc/cni/net.d"

	p := PodmanTest{
		PodmanBinary:        podmanBinary,
		ConmonBinary:        conmonBinary,
		CrioRoot:            filepath.Join(tempDir, "crio"),
		CNIConfigDir:        CNIConfigDir,
		RunCBinary:          runCBinary,
		RunRoot:             filepath.Join(tempDir, "crio-run"),
		StorageOptions:      storageOptions,
		SignaturePolicyPath: filepath.Join(INTEGRATION_ROOT, "test/policy.json"),
		ArtifactPath:        ARTIFACT_DIR,
		TempDir:             tempDir,
		CgroupManager:       cgroupManager,
	}

	// Setup registries.conf ENV variable
	p.setDefaultRegistriesConfigEnv()
	return p
}

//MakeOptions assembles all the podman main options
func (p *PodmanTest) MakeOptions() []string {
	return strings.Split(fmt.Sprintf("--root %s --runroot %s --runtime %s --conmon %s --cni-config-dir %s --cgroup-manager %s",
		p.CrioRoot, p.RunRoot, p.RunCBinary, p.ConmonBinary, p.CNIConfigDir, p.CgroupManager), " ")
}

// Podman is the exec call to podman on the filesystem, uid and gid the credentials to use
func (p *PodmanTest) PodmanAsUser(args []string, uid, gid uint32, env []string) *PodmanSession {
	podmanOptions := p.MakeOptions()
	if os.Getenv("HOOK_OPTION") != "" {
		podmanOptions = append(podmanOptions, os.Getenv("HOOK_OPTION"))
	}
	podmanOptions = append(podmanOptions, strings.Split(p.StorageOptions, " ")...)
	podmanOptions = append(podmanOptions, args...)
	if env == nil {
		fmt.Printf("Running: %s %s\n", p.PodmanBinary, strings.Join(podmanOptions, " "))
	} else {
		fmt.Printf("Running: (env: %v) %s %s\n", env, p.PodmanBinary, strings.Join(podmanOptions, " "))
	}
	var command *exec.Cmd

	if uid != 0 || gid != 0 {
		nsEnterOpts := append([]string{"--userspec", fmt.Sprintf("%d:%d", uid, gid), "/", p.PodmanBinary}, podmanOptions...)
		command = exec.Command("chroot", nsEnterOpts...)
	} else {
		command = exec.Command(p.PodmanBinary, podmanOptions...)
	}
	if env != nil {
		command.Env = env
	}

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run podman command: %s\n%v", strings.Join(podmanOptions, " "), err))
	}
	return &PodmanSession{session}
}

// Podman is the exec call to podman on the filesystem
func (p *PodmanTest) Podman(args []string) *PodmanSession {
	return p.PodmanAsUser(args, 0, 0, nil)
}

//WaitForContainer waits on a started container
func WaitForContainer(p *PodmanTest) bool {
	for i := 0; i < 10; i++ {
		if p.NumberOfRunningContainers() == 1 {
			return true
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

// Cleanup cleans up the temporary store
func (p *PodmanTest) Cleanup() {
	// Remove all containers
	stopall := p.Podman([]string{"stop", "-a", "--timeout", "0"})
	stopall.WaitWithDefaultTimeout()

	session := p.Podman([]string{"rm", "-fa"})
	session.Wait(90)
	// Nuke tempdir
	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}

	// Clean up the registries configuration file ENV variable set in Create
	resetRegistriesConfigEnv()
}

// CleanupPod cleans up the temporary store
func (p *PodmanTest) CleanupPod() {
	// Remove all containers
	session := p.Podman([]string{"pod", "rm", "-fa"})
	session.Wait(90)
	// Nuke tempdir
	if err := os.RemoveAll(p.TempDir); err != nil {
		fmt.Printf("%q\n", err)
	}
}

// GrepString takes session output and behaves like grep. it returns a bool
// if successful and an array of strings on positive matches
func (s *PodmanSession) GrepString(term string) (bool, []string) {
	var (
		greps   []string
		matches bool
	)

	for _, line := range strings.Split(s.OutputToString(), "\n") {
		if strings.Contains(line, term) {
			matches = true
			greps = append(greps, line)
		}
	}
	return matches, greps
}

// Pull Images pulls multiple images
func (p *PodmanTest) PullImages(images []string) error {
	for _, i := range images {
		p.PullImage(i)
	}
	return nil
}

// Pull Image a single image
// TODO should the timeout be configurable?
func (p *PodmanTest) PullImage(image string) error {
	session := p.Podman([]string{"pull", image})
	session.Wait(60)
	Expect(session.ExitCode()).To(Equal(0))
	return nil
}

// OutputToString formats session output to string
func (s *PodmanSession) OutputToString() string {
	fields := strings.Fields(fmt.Sprintf("%s", s.Out.Contents()))
	return strings.Join(fields, " ")
}

// OutputToStringArray returns the output as a []string
// where each array item is a line split by newline
func (s *PodmanSession) OutputToStringArray() []string {
	output := fmt.Sprintf("%s", s.Out.Contents())
	return strings.Split(output, "\n")
}

// ErrorGrepString takes session stderr output and behaves like grep. it returns a bool
// if successful and an array of strings on positive matches
func (s *PodmanSession) ErrorGrepString(term string) (bool, []string) {
	var (
		greps   []string
		matches bool
	)

	for _, line := range strings.Split(s.ErrorToString(), "\n") {
		if strings.Contains(line, term) {
			matches = true
			greps = append(greps, line)
		}
	}
	return matches, greps
}

// ErrorToString formats session stderr to string
func (s *PodmanSession) ErrorToString() string {
	fields := strings.Fields(fmt.Sprintf("%s", s.Err.Contents()))
	return strings.Join(fields, " ")
}

// ErrorToStringArray returns the stderr output as a []string
// where each array item is a line split by newline
func (s *PodmanSession) ErrorToStringArray() []string {
	output := fmt.Sprintf("%s", s.Err.Contents())
	return strings.Split(output, "\n")
}

// IsJSONOutputValid attempts to unmarshal the session buffer
// and if successful, returns true, else false
func (s *PodmanSession) IsJSONOutputValid() bool {
	var i interface{}
	if err := json.Unmarshal(s.Out.Contents(), &i); err != nil {
		fmt.Println(err)
		return false
	}
	return true
}

// InspectContainerToJSON takes the session output of an inspect
// container and returns json
func (s *PodmanSession) InspectContainerToJSON() []inspect.ContainerData {
	var i []inspect.ContainerData
	err := json.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

// InspectPodToJSON takes the sessions output from a pod inspect and returns json
func (s *PodmanSession) InspectPodToJSON() libpod.PodInspect {
	var i libpod.PodInspect
	err := json.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

// InspectImageJSON takes the session output of an inspect
// image and returns json
func (s *PodmanSession) InspectImageJSON() []inspect.ImageData {
	var i []inspect.ImageData
	err := json.Unmarshal(s.Out.Contents(), &i)
	Expect(err).To(BeNil())
	return i
}

func (s *PodmanSession) WaitWithDefaultTimeout() {
	s.Wait(defaultWaitTimeout)
	fmt.Println("output:", s.OutputToString())
}

// SystemExec is used to exec a system command to check its exit code or output
func (p *PodmanTest) SystemExec(command string, args []string) *PodmanSession {
	c := exec.Command(command, args...)
	session, err := gexec.Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run command: %s %s", command, strings.Join(args, " ")))
	}
	return &PodmanSession{session}
}

// CreateArtifact creates a cached image in the artifact dir
func (p *PodmanTest) CreateArtifact(image string) error {
	if os.Getenv("NO_TEST_CACHE") != "" {
		return nil
	}
	fmt.Printf("Caching %s...", image)
	dest := strings.Split(image, "/")
	destName := fmt.Sprintf("/tmp/%s.tar", strings.Replace(strings.Join(strings.Split(dest[len(dest)-1], "/"), ""), ":", "-", -1))
	if _, err := os.Stat(destName); os.IsNotExist(err) {
		pull := p.Podman([]string{"pull", image})
		pull.Wait(90)

		save := p.Podman([]string{"save", "-o", destName, image})
		save.Wait(90)
		fmt.Printf("\n")
	} else {
		fmt.Printf(" already exists.\n")
	}
	return nil
}

// RestoreArtifact puts the cached image into our test store
func (p *PodmanTest) RestoreArtifact(image string) error {
	fmt.Printf("Restoring %s...\n", image)
	dest := strings.Split(image, "/")
	destName := fmt.Sprintf("/tmp/%s.tar", strings.Replace(strings.Join(strings.Split(dest[len(dest)-1], "/"), ""), ":", "-", -1))
	restore := p.Podman([]string{"load", "-q", "-i", destName})
	restore.Wait(90)
	return nil
}

// RestoreAllArtifacts unpacks all cached images
func (p *PodmanTest) RestoreAllArtifacts() error {
	if os.Getenv("NO_TEST_CACHE") != "" {
		return nil
	}
	for _, image := range RESTORE_IMAGES {
		if err := p.RestoreArtifact(image); err != nil {
			return err
		}
	}
	return nil
}

// CreatePod creates a pod with no infra container
// it optionally takes a pod name
func (p *PodmanTest) CreatePod(name string) (*PodmanSession, int, string) {
	var podmanArgs = []string{"pod", "create", "--infra=false", "--share", ""}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

//RunTopContainer runs a simple container in the background that
// runs top.  If the name passed != "", it will have a name
func (p *PodmanTest) RunTopContainer(name string) *PodmanSession {
	var podmanArgs = []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "top")
	return p.Podman(podmanArgs)
}

func (p *PodmanTest) RunTopContainerInPod(name, pod string) *PodmanSession {
	var podmanArgs = []string{"run", "--pod", pod}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "top")
	return p.Podman(podmanArgs)
}

//RunLsContainer runs a simple container in the background that
// simply runs ls. If the name passed != "", it will have a name
func (p *PodmanTest) RunLsContainer(name string) (*PodmanSession, int, string) {
	var podmanArgs = []string{"run"}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "ls")
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

func (p *PodmanTest) RunLsContainerInPod(name, pod string) (*PodmanSession, int, string) {
	var podmanArgs = []string{"run", "--pod", pod}
	if name != "" {
		podmanArgs = append(podmanArgs, "--name", name)
	}
	podmanArgs = append(podmanArgs, "-d", ALPINE, "ls")
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session, session.ExitCode(), session.OutputToString()
}

//NumberOfContainersRunning returns an int of how many
// containers are currently running.
func (p *PodmanTest) NumberOfContainersRunning() int {
	var containers []string
	ps := p.Podman([]string{"ps", "-q"})
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			containers = append(containers, i)
		}
	}
	return len(containers)
}

// NumberOfContainers returns an int of how many
// containers are currently defined.
func (p *PodmanTest) NumberOfContainers() int {
	var containers []string
	ps := p.Podman([]string{"ps", "-aq"})
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			containers = append(containers, i)
		}
	}
	return len(containers)
}

// NumberOfPods returns an int of how many
// pods are currently defined.
func (p *PodmanTest) NumberOfPods() int {
	var pods []string
	ps := p.Podman([]string{"pod", "ps", "-q"})
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			pods = append(pods, i)
		}
	}
	return len(pods)
}

// NumberOfRunningContainers returns an int of how many containers are currently
// running
func (p *PodmanTest) NumberOfRunningContainers() int {
	var containers []string
	ps := p.Podman([]string{"ps", "-q"})
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			containers = append(containers, i)
		}
	}
	return len(containers)
}

// StringInSlice determines if a string is in a string slice, returns bool
func StringInSlice(s string, sl []string) bool {
	for _, i := range sl {
		if i == s {
			return true
		}
	}
	return false
}

//LineInOutputStartsWith returns true if a line in a
// session output starts with the supplied string
func (s *PodmanSession) LineInOuputStartsWith(term string) bool {
	for _, i := range s.OutputToStringArray() {
		if strings.HasPrefix(i, term) {
			return true
		}
	}
	return false
}

//LineInOutputContains returns true if a line in a
// session output starts with the supplied string
func (s *PodmanSession) LineInOutputContains(term string) bool {
	for _, i := range s.OutputToStringArray() {
		if strings.Contains(i, term) {
			return true
		}
	}
	return false
}

//tagOutPutToMap parses each string in imagesOutput and returns
// a map of repo:tag pairs.  Notice, the first array item will
// be skipped as it's considered to be the header.
func tagOutputToMap(imagesOutput []string) map[string]string {
	m := make(map[string]string)
	// iterate over output but skip the header
	for _, i := range imagesOutput[1:] {
		tmp := []string{}
		for _, x := range strings.Split(i, " ") {
			if x != "" {
				tmp = append(tmp, x)
			}
		}
		// podman-images(1) return a list like output
		// in the format of "Repository Tag [...]"
		if len(tmp) < 2 {
			continue
		}
		m[tmp[0]] = tmp[1]
	}
	return m
}

//LineInOutputContainsTag returns true if a line in the
// session's output contains the repo-tag pair as returned
// by podman-images(1).
func (s *PodmanSession) LineInOutputContainsTag(repo, tag string) bool {
	tagMap := tagOutputToMap(s.OutputToStringArray())
	for r, t := range tagMap {
		if repo == r && tag == t {
			return true
		}
	}
	return false
}

//GetContainerStatus returns the containers state.
// This function assumes only one container is active.
func (p *PodmanTest) GetContainerStatus() string {
	var podmanArgs = []string{"ps"}
	podmanArgs = append(podmanArgs, "--all", "--format={{.Status}}")
	session := p.Podman(podmanArgs)
	session.WaitWithDefaultTimeout()
	return session.OutputToString()
}

// BuildImage uses podman build and buildah to build an image
// called imageName based on a string dockerfile
func (p *PodmanTest) BuildImage(dockerfile, imageName string, layers string) {
	dockerfilePath := filepath.Join(p.TempDir, "Dockerfile")
	err := ioutil.WriteFile(dockerfilePath, []byte(dockerfile), 0755)
	Expect(err).To(BeNil())
	session := p.Podman([]string{"build", "--layers=" + layers, "-t", imageName, "--file", dockerfilePath, p.TempDir})
	session.Wait(120)
	Expect(session.ExitCode()).To(Equal(0))
}

//GetHostDistributionInfo returns a struct with its distribution name and version
func GetHostDistributionInfo() HostOS {
	f, err := os.Open("/etc/os-release")
	defer f.Close()
	if err != nil {
		return HostOS{}
	}

	l := bufio.NewScanner(f)
	host := HostOS{}
	for l.Scan() {
		if strings.HasPrefix(l.Text(), "ID=") {
			host.Distribution = strings.Replace(strings.TrimSpace(strings.Join(strings.Split(l.Text(), "=")[1:], "")), "\"", "", -1)
		}
		if strings.HasPrefix(l.Text(), "VERSION_ID=") {
			host.Version = strings.Replace(strings.TrimSpace(strings.Join(strings.Split(l.Text(), "=")[1:], "")), "\"", "", -1)
		}
	}
	return host
}

func (p *PodmanTest) setDefaultRegistriesConfigEnv() {
	defaultFile := filepath.Join(INTEGRATION_ROOT, "test/registries.conf")
	os.Setenv("REGISTRIES_CONFIG_PATH", defaultFile)
}

func (p *PodmanTest) setRegistriesConfigEnv(b []byte) {
	outfile := filepath.Join(p.TempDir, "registries.conf")
	os.Setenv("REGISTRIES_CONFIG_PATH", outfile)
	ioutil.WriteFile(outfile, b, 0644)
}

func resetRegistriesConfigEnv() {
	os.Setenv("REGISTRIES_CONFIG_PATH", "")
}

// IsKernelNewThan compares the current kernel version to one provided.  If
// the kernel is equal to or greater, returns true
func IsKernelNewThan(version string) (bool, error) {
	inputVersion, err := kernel.ParseRelease(version)
	if err != nil {
		return false, err
	}
	kv, err := kernel.GetKernelVersion()
	if err == nil {
		return false, err
	}
	// CompareKernelVersion compares two kernel.VersionInfo structs.
	// Returns -1 if a < b, 0 if a == b, 1 it a > b
	result := kernel.CompareKernelVersion(*kv, *inputVersion)
	if result >= 0 {
		return true, nil
	}
	return false, nil

}

//Wait process or service inside container start, and ready to be used.
func WaitContainerReady(p *PodmanTest, id string, expStr string, timeout int, step int) bool {
	startTime := time.Now()
	s := p.Podman([]string{"logs", id})
	s.WaitWithDefaultTimeout()
	fmt.Println(startTime)
	for {
		if time.Since(startTime) >= time.Duration(timeout)*time.Second {
			return false
		}
		if strings.Contains(s.OutputToString(), expStr) {
			return true
		}
		time.Sleep(time.Duration(step) * time.Second)
		s = p.Podman([]string{"logs", id})
		s.WaitWithDefaultTimeout()
	}
}

//IsCommandAvaible check if command exist
func IsCommandAvailable(command string) bool {
	check := exec.Command("bash", "-c", strings.Join([]string{"command -v", command}, " "))
	err := check.Run()
	if err != nil {
		return false
	}
	return true
}

// WriteJsonFile write json format data to a json file
func WriteJsonFile(data []byte, filePath string) error {
	var jsonData map[string]interface{}
	json.Unmarshal(data, &jsonData)
	formatJson, _ := json.MarshalIndent(jsonData, "", "	")
	return ioutil.WriteFile(filePath, formatJson, 0644)
}

func getTestContext() context.Context {
	return context.Background()
}

func containerized() bool {
	container := os.Getenv("container")
	if container != "" {
		return true
	}
	b, err := ioutil.ReadFile("/proc/1/cgroup")
	if err != nil {
		// shrug, if we cannot read that file, return false
		return false
	}
	if strings.Index(string(b), "docker") > -1 {
		return true
	}
	return false
}
