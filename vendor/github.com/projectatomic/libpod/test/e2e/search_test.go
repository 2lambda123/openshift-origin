package integration

import (
	"fmt"
	"os"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman search", func() {
	var (
		tempdir    string
		err        error
		podmanTest PodmanTest
	)
	const regFileContents = `
	[registries.search]
	registries = ['localhost:5000']

	[registries.insecure]
	registries = ['localhost:5000']`

	const badRegFileContents = `
	[registries.search]
	registries = ['localhost:5000']
    # empty
	[registries.insecure]
	registries = []`

	const regFileContents2 = `
	[registries.search]
	registries = ['localhost:5000', 'localhost:6000']

	[registries.insecure]
	registries = ['localhost:5000']`
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

	It("podman search", func() {
		search := podmanTest.Podman([]string{"search", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
	})

	It("podman search single registry flag", func() {
		search := podmanTest.Podman([]string{"search", "registry.fedoraproject.org/fedora-minimal"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.LineInOutputContains("fedoraproject.org/fedora-minimal")).To(BeTrue())
	})

	It("podman search format flag", func() {
		search := podmanTest.Podman([]string{"search", "--format", "table {{.Index}} {{.Name}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
	})

	It("podman search no-trunc flag", func() {
		search := podmanTest.Podman([]string{"search", "--no-trunc", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(BeNumerically(">", 1))
		Expect(search.LineInOutputContains("docker.io/library/alpine")).To(BeTrue())
		Expect(search.LineInOutputContains("...")).To(BeFalse())
	})

	It("podman search limit flag", func() {
		search := podmanTest.Podman([]string{"search", "--limit", "3", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		Expect(len(search.OutputToStringArray())).To(Equal(4))
	})

	It("podman search with filter stars", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "stars=10", "--format", "{{.Stars}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(strconv.Atoi(output[i])).To(BeNumerically(">=", 10))
		}
	})

	It("podman search with filter is-official", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "is-official", "--format", "{{.Official}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(output[i]).To(Equal("[OK]"))
		}
	})

	It("podman search with filter is-automated", func() {
		search := podmanTest.Podman([]string{"search", "--filter", "is-automated=false", "--format", "{{.Automated}}", "alpine"})
		search.WaitWithDefaultTimeout()
		Expect(search.ExitCode()).To(Equal(0))
		output := search.OutputToStringArray()
		for i := 0; i < len(output); i++ {
			Expect(output[i]).To(Equal(""))
		}
	})

	It("podman search attempts HTTP if tls-verify flag is set false", func() {
		podmanTest.RestoreArtifact(registry)
		fakereg := podmanTest.Podman([]string{"run", "-d", "--name", "registry", "-p", "5000:5000", registry, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		fakereg.WaitWithDefaultTimeout()
		Expect(fakereg.ExitCode()).To(Equal(0))

		if !WaitContainerReady(&podmanTest, "registry", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}

		search := podmanTest.Podman([]string{"search", "localhost:5000/fake/image:andtag", "--tls-verify=false"})
		search.WaitWithDefaultTimeout()

		// if this test succeeded, there will be no output (there is no entry named fake/image:andtag in an empty registry)
		// and the exit code will be 0
		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).Should(BeEmpty())
		Expect(search.ErrorToString()).Should(BeEmpty())
	})

	It("podman search in local registry", func() {
		podmanTest.RestoreArtifact(registry)
		registry := podmanTest.Podman([]string{"run", "-d", "--name", "registry3", "-p", "5000:5000", registry, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		registry.WaitWithDefaultTimeout()
		Expect(registry.ExitCode()).To(Equal(0))

		if !WaitContainerReady(&podmanTest, "registry3", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}

		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))
		search := podmanTest.Podman([]string{"search", "localhost:5000/my-alpine", "--tls-verify=false"})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).ShouldNot(BeEmpty())
	})

	It("podman search attempts HTTP if registry is in registries.insecure and force secure is false", func() {
		podmanTest.RestoreArtifact(registry)
		registry := podmanTest.Podman([]string{"run", "-d", "--name", "registry4", "-p", "5000:5000", registry, "/entrypoint.sh", "/etc/docker/registry/config.yml"})
		registry.WaitWithDefaultTimeout()
		Expect(registry.ExitCode()).To(Equal(0))

		if !WaitContainerReady(&podmanTest, "registry4", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}

		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		// registries.conf set up
		podmanTest.setRegistriesConfigEnv([]byte(regFileContents))

		search := podmanTest.Podman([]string{"search", "localhost:5000/my-alpine"})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		match, _ := search.GrepString("my-alpine")
		Expect(match).Should(BeTrue())
		Expect(search.ErrorToString()).Should(BeEmpty())

		// cleanup
		resetRegistriesConfigEnv()
	})

	It("podman search doesn't attempt HTTP if force secure is true", func() {
		podmanTest.RestoreArtifact(registry)
		registry := podmanTest.Podman([]string{"run", "-d", "-p", "5000:5000", "--name", "registry5", registry})
		registry.WaitWithDefaultTimeout()
		Expect(registry.ExitCode()).To(Equal(0))

		if !WaitContainerReady(&podmanTest, "registry5", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}
		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		// registries.conf set up
		podmanTest.setRegistriesConfigEnv([]byte(regFileContents))

		search := podmanTest.Podman([]string{"search", "localhost:5000/my-alpine", "--tls-verify=true"})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).Should(BeEmpty())
		match, _ := search.ErrorGrepString("error")
		Expect(match).Should(BeTrue())

		// cleanup
		resetRegistriesConfigEnv()
	})

	It("podman search doesn't attempt HTTP if registry is not listed as insecure", func() {
		podmanTest.RestoreArtifact(registry)
		registry := podmanTest.Podman([]string{"run", "-d", "-p", "5000:5000", "--name", "registry6", registry})
		registry.WaitWithDefaultTimeout()
		Expect(registry.ExitCode()).To(Equal(0))

		if !WaitContainerReady(&podmanTest, "registry6", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}
		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:5000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		// registries.conf set up
		podmanTest.setRegistriesConfigEnv([]byte(badRegFileContents))

		search := podmanTest.Podman([]string{"search", "localhost:5000/my-alpine"})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).Should(BeEmpty())
		match, _ := search.ErrorGrepString("error")
		Expect(match).Should(BeTrue())

		// cleanup
		resetRegistriesConfigEnv()
	})

	It("podman search doesn't attempt HTTP if one registry is not listed as insecure", func() {
		podmanTest.RestoreArtifact(registry)
		registryLocal := podmanTest.Podman([]string{"run", "-d", "-p", "5000:5000", "--name", "registry7", registry})
		registryLocal.WaitWithDefaultTimeout()
		Expect(registryLocal.ExitCode()).To(Equal(0))

		if !WaitContainerReady(&podmanTest, "registry7", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}

		registryLocal = podmanTest.Podman([]string{"run", "-d", "-p", "6000:5000", "--name", "registry8", registry})
		registryLocal.WaitWithDefaultTimeout()
		Expect(registryLocal.ExitCode()).To(Equal(0))

		if !WaitContainerReady(&podmanTest, "registry8", "listening on", 20, 1) {
			Skip("Can not start docker registry.")
		}
		push := podmanTest.Podman([]string{"push", "--tls-verify=false", "--remove-signatures", ALPINE, "localhost:6000/my-alpine"})
		push.WaitWithDefaultTimeout()
		Expect(push.ExitCode()).To(Equal(0))

		// registries.conf set up
		podmanTest.setRegistriesConfigEnv([]byte(regFileContents2))

		search := podmanTest.Podman([]string{"search", "my-alpine"})
		search.WaitWithDefaultTimeout()

		Expect(search.ExitCode()).To(Equal(0))
		Expect(search.OutputToString()).Should(BeEmpty())
		match, _ := search.ErrorGrepString("error")
		Expect(match).Should(BeTrue())

		// cleanup
		resetRegistriesConfigEnv()
	})
})
