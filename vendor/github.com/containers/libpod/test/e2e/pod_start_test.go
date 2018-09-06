package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman pod start", func() {
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

	It("podman pod start bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "start", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman pod start single empty pod", func() {
		_, ec, podid := podmanTest.CreatePod("")
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "start", podid})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
	})

	It("podman pod start single pod by name", func() {
		_, ec, _ := podmanTest.CreatePod("foobar99")
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman pod start multiple pods", func() {
		_, ec, podid1 := podmanTest.CreatePod("foobar99")
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		_, ec2, podid2 := podmanTest.CreatePod("foobar100")
		Expect(ec2).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", podid1, podid2})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("podman pod start all pods", func() {
		_, ec, _ := podmanTest.CreatePod("foobar99")
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		_, ec, _ = podmanTest.CreatePod("foobar100")
		Expect(ec).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("podman pod start latest pod", func() {
		_, ec, _ := podmanTest.CreatePod("foobar99")
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		_, ec, _ = podmanTest.CreatePod("foobar100")
		Expect(ec).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", "--latest"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod start multiple pods with bogus", func() {
		_, ec, podid := podmanTest.CreatePod("foobar99")
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		session = podmanTest.Podman([]string{"pod", "start", podid, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(125))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})
})
