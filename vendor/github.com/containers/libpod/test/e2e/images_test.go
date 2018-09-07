package integration

import (
	"fmt"
	"os"
	"sort"

	"github.com/docker/go-units"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman images", func() {
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
	It("podman images", func() {
		session := podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		Expect(session.LineInOuputStartsWith("docker.io/library/alpine")).To(BeTrue())
		Expect(session.LineInOuputStartsWith("docker.io/library/busybox")).To(BeTrue())
	})

	It("podman images with multiple tags", func() {
		// tag "docker.io/library/alpine:latest" to "foo:{a,b,c}"
		session := podmanTest.Podman([]string{"tag", ALPINE, "foo:a", "foo:b", "foo:c"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// tag "foo:c" to "bar:{a,b}"
		session = podmanTest.Podman([]string{"tag", "foo:c", "bar:a", "bar:b"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		// check all previous and the newly tagged images
		session = podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		session.LineInOutputContainsTag("docker.io/library/alpine", "latest")
		session.LineInOutputContainsTag("docker.io/library/busybox", "glibc")
		session.LineInOutputContainsTag("foo", "a")
		session.LineInOutputContainsTag("foo", "b")
		session.LineInOutputContainsTag("foo", "c")
		session.LineInOutputContainsTag("bar", "a")
		session.LineInOutputContainsTag("bar", "b")
	})

	It("podman images with digests", func() {
		session := podmanTest.Podman([]string{"images", "--digests"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 2))
		Expect(session.LineInOuputStartsWith("docker.io/library/alpine")).To(BeTrue())
		Expect(session.LineInOuputStartsWith("docker.io/library/busybox")).To(BeTrue())
	})

	It("podman images in JSON format", func() {
		session := podmanTest.Podman([]string{"images", "--format=json"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
	})

	It("podman images in GO template format", func() {
		session := podmanTest.Podman([]string{"images", "--format={{.ID}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
	})

	It("podman images with short options", func() {
		session := podmanTest.Podman([]string{"images", "-qn"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman images filter by image name", func() {
		session := podmanTest.Podman([]string{"images", "-q", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(1))
	})

	It("podman images filter before image", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.Podman([]string{"images", "-q", "-f", "before=foobar.com/before:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(2))
	})

	It("podman images filter after image", func() {
		rmi := podmanTest.Podman([]string{"rmi", "busybox"})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi.ExitCode()).To(Equal(0))

		dockerfile := `FROM docker.io/library/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.Podman([]string{"images", "-q", "-f", "after=docker.io/library/alpine:latest"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(1))
	})

	It("podman images filter dangling", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
`
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		podmanTest.BuildImage(dockerfile, "foobar.com/before:latest", "false")
		result := podmanTest.Podman([]string{"images", "-q", "-f", "dangling=true"})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
		Expect(len(result.OutputToStringArray())).To(Equal(1))
	})

	It("podman check for image with sha256: prefix", func() {
		session := podmanTest.Podman([]string{"inspect", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(session.IsJSONOutputValid()).To(BeTrue())
		imageData := session.InspectImageJSON()

		result := podmanTest.Podman([]string{"images", fmt.Sprintf("sha256:%s", imageData[0].ID)})
		result.WaitWithDefaultTimeout()
		Expect(result.ExitCode()).To(Equal(0))
	})

	It("podman images sort by tag", func() {
		session := podmanTest.Podman([]string{"images", "--sort", "tag", "--format={{.Tag}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		sortedArr := session.OutputToStringArray()
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool { return sortedArr[i] < sortedArr[j] })).To(BeTrue())
	})

	It("podman images sort by size", func() {
		session := podmanTest.Podman([]string{"images", "--sort", "size", "--format={{.Size}}"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))

		sortedArr := session.OutputToStringArray()
		Expect(sort.SliceIsSorted(sortedArr, func(i, j int) bool {
			size1, _ := units.FromHumanSize(sortedArr[i])
			size2, _ := units.FromHumanSize(sortedArr[j])
			return size1 < size2
		})).To(BeTrue())
	})

	It("podman images --all flag", func() {
		dockerfile := `FROM docker.io/library/alpine:latest
RUN mkdir hello
RUN touch test.txt
ENV foo=bar
`
		podmanTest.BuildImage(dockerfile, "test", "true")
		session := podmanTest.Podman([]string{"images"})
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).To(Equal(0))
		Expect(len(session.OutputToStringArray())).To(Equal(4))

		session2 := podmanTest.Podman([]string{"images", "--all"})
		session2.WaitWithDefaultTimeout()
		Expect(session2.ExitCode()).To(Equal(0))
		Expect(len(session2.OutputToStringArray())).To(Equal(6))
	})
})
