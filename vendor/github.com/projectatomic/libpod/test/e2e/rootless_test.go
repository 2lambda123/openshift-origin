package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman rootless", func() {
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
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		timedResult := fmt.Sprintf("Test: %s completed in %f seconds", f.TestText, f.Duration.Seconds())
		GinkgoWriter.Write([]byte(timedResult))
	})

	It("podman rootless rootfs", func() {
		// Check if we can create an user namespace
		err := exec.Command("unshare", "-r", "echo", "hello").Run()
		if err != nil {
			Skip("User namespaces not supported.")
		}

		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup.ExitCode()).To(Equal(0))
		cid := setup.OutputToString()

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount.ExitCode()).To(Equal(0))
		mountPath := mount.OutputToString()

		chownFunc := func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return os.Lchown(p, 1000, 1000)
		}

		err = filepath.Walk(tempdir, chownFunc)
		if err != nil {
			fmt.Printf("cannot chown the directory: %q\n", err)
			os.Exit(1)
		}

		runRootless := func(mountPath string) {
			tempdir, err := CreateTempDirInTempDir()
			Expect(err).To(BeNil())
			podmanTest := PodmanCreate(tempdir)
			err = filepath.Walk(tempdir, chownFunc)
			Expect(err).To(BeNil())

			xdgRuntimeDir, err := ioutil.TempDir("/run", "")
			Expect(err).To(BeNil())
			defer os.RemoveAll(xdgRuntimeDir)
			err = filepath.Walk(xdgRuntimeDir, chownFunc)
			Expect(err).To(BeNil())

			home, err := CreateTempDirInTempDir()
			Expect(err).To(BeNil())
			err = filepath.Walk(xdgRuntimeDir, chownFunc)
			Expect(err).To(BeNil())

			env := os.Environ()
			env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir))
			env = append(env, fmt.Sprintf("HOME=%s", home))
			env = append(env, "PODMAN_ALLOW_SINGLE_ID_MAPPING_IN_USERNS=1")
			cmd := podmanTest.PodmanAsUser([]string{"run", "--rootfs", mountPath, "echo", "hello"}, 1000, 1000, env)
			cmd.WaitWithDefaultTimeout()
			Expect(cmd.LineInOutputContains("hello")).To(BeTrue())
			Expect(cmd.ExitCode()).To(Equal(0))
		}

		runRootless(mountPath)

		umount := podmanTest.Podman([]string{"umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount.ExitCode()).To(Equal(0))
	})
})
