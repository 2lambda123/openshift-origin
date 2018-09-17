package integration

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman run with --cgroup-parent", func() {
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
		podmanTest.RestoreArtifact(fedoraMinimal)
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	Specify("valid --cgroup-parent using cgroupfs", func() {
		if !containerized() {
			Skip("Must be containerized to run this test.")
		}
		cgroup := "/zzz"
		run := podmanTest.Podman([]string{"run", "--cgroup-parent", cgroup, fedoraMinimal, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run.ExitCode()).To(Equal(0))
		ok, _ := run.GrepString(cgroup)
		Expect(ok).To(BeTrue())
	})

	Specify("no --cgroup-parent", func() {
		cgroup := "/libpod_parent"
		if !containerized() {
			cgroup = "/machine.slice"
		}
		run := podmanTest.Podman([]string{"run", fedoraMinimal, "cat", "/proc/self/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run.ExitCode()).To(Equal(0))
		ok, _ := run.GrepString(cgroup)
		Expect(ok).To(BeTrue())
	})

	Specify("valid --cgroup-parent using slice", func() {
		if containerized() {
			Skip("Requires Systemd cgroup manager support")
		}
		cgroup := "aaaa.slice"
		run := podmanTest.Podman([]string{"run", "--cgroup-parent", cgroup, fedoraMinimal, "cat", "/proc/1/cgroup"})
		run.WaitWithDefaultTimeout()
		Expect(run.ExitCode()).To(Equal(0))
		ok, _ := run.GrepString(cgroup)
		Expect(ok).To(BeTrue())
	})
})
