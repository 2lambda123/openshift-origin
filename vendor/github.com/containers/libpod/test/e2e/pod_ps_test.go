package integration

import (
	"fmt"
	"os"
	"sort"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman ps", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)

	BeforeEach(func() {
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanCreate(tempdir)
		podmanTest.RestoreAllArtifacts()
	})

	AfterEach(func() {
		podmanTest.CleanupPod()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman pod ps no pods", func() {
		session := podmanTest.Podman([]string{"pod", "ps"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pod ps default", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))
	})

	It("podman pod ps quiet flag", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		_, ec, _ = podmanTest.RunLsContainerInPod("", podid)
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps", "-q"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))
		Expect(podid).To(ContainSubstring(result.OutputToStringArray()[0]))
	})

	It("podman pod ps no-trunc", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		_, ec2, _ := podmanTest.RunLsContainerInPod("", podid)
		Expect(ec2).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).Should(BeNumerically(">", 0))
		Expect(podid).To(Equal(result.OutputToStringArray()[0]))
	})

	It("podman pod ps latest", func() {
		_, ec, podid1 := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		_, ec2, podid2 := podmanTest.CreatePod("")
		Expect(ec2).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(result.OutputToString()).To(ContainSubstring(podid2))
		Expect(result.OutputToString()).To(Not(ContainSubstring(podid1)))
	})
	It("podman pod ps id filter flag", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "ps", "--filter", fmt.Sprintf("id=%s", podid)})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman pod ps mutually exclusive flags", func() {
		session := podmanTest.Podman([]string{"pod", "ps", "-q", "--format", "{{.ID}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Not(Equal(0)))

	})

	It("podman pod ps --sort by name", func() {
		_, ec, _ := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		_, ec2, _ := podmanTest.CreatePod("")
		Expect(ec2).To(Equal(0))

		_, ec3, _ := podmanTest.CreatePod("")
		Expect(ec3).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "ps", "--sort=name", "--format", "{{.Name}}"})

		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		sortedArr := session.OutputToStringArray()

		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).To(BeTrue())
	})

	It("podman pod ps --ctr-names", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		_, ec, _ = podmanTest.RunLsContainerInPod("test2", podid)
		Expect(ec).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "ps", "--format={{.ContainerInfo}}", "--ctr-names"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring("test1"))
		Expect(session.OutputToString()).To(ContainSubstring("test2"))
	})

	It("podman pod ps filter ctr attributes", func() {
		_, ec, podid1 := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		_, ec2, podid2 := podmanTest.CreatePod("")
		Expect(ec2).To(Equal(0))

		_, ec3, cid := podmanTest.RunLsContainerInPod("test2", podid2)
		Expect(ec3).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-names=test1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(podid1))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid2)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", fmt.Sprintf("ctr-ids=%s", cid)})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(podid2))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid1)))

		_, ec3, podid3 := podmanTest.CreatePod("")
		Expect(ec3).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-number=1"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(podid1))
		Expect(session.OutputToString()).To(ContainSubstring(podid2))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid3)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-status=running"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(podid1))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid2)))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid3)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-status=exited"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(ContainSubstring(podid2))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid1)))
		Expect(session.OutputToString()).To(Not(ContainSubstring(podid3)))

		session = podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc", "--filter", "ctr-status=created"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.OutputToString()).To(BeEmpty())
	})
})
