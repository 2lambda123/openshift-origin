package builds

import (
	"fmt"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildv1 "github.com/openshift/api/build/v1"
	buildutil "github.com/openshift/origin/pkg/build/util"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/jenkins"
)

const (
	localClientPluginSnapshotImageStream = "jenkins-client-plugin-snapshot-test"
	localClientPluginSnapshotImage       = "openshift/" + localClientPluginSnapshotImageStream + ":latest"
	localSyncPluginSnapshotImageStream   = "jenkins-sync-plugin-snapshot-test"
	localSyncPluginSnapshotImage         = "openshift/" + localSyncPluginSnapshotImageStream + ":latest"
	clientLicenseText                    = "About OpenShift Client Jenkins Plugin"
	syncLicenseText                      = "About OpenShift Sync"
	clientPluginName                     = "openshift-client"
	syncPluginName                       = "openshift-sync"
	secretName                           = "secret-to-credential"
	secretCredentialSyncLabel            = "credential.sync.jenkins.openshift.io"
	envVarsPipelineGitRepoBuildConfig    = "test-build-app-pipeline"
)

var _ = g.Describe("[Feature:Builds][Slow] openshift pipeline build", func() {
	defer g.GinkgoRecover()

	var (
		jenkinsEphemeralTemplatePath  = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json")
		jenkinsPersistentTemplatePath = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-persistent-template.json")
		envVarsPipelinePath           = exutil.FixturePath("testdata", "samplepipeline-withenvs.yaml")
		origPipelinePath              = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "samplepipeline.yaml")
		configMapPodTemplatePath      = exutil.FixturePath("testdata", "config-map-jenkins-slave-pods.yaml")
		imagestreamPodTemplatePath    = exutil.FixturePath("testdata", "imagestream-jenkins-slave-pods.yaml")
		imagestreamtagPodTemplatePath = exutil.FixturePath("testdata", "imagestreamtag-jenkins-slave-pods.yaml")
		podTemplateSlavePipelinePath  = exutil.FixturePath("testdata", "jenkins-slave-template.yaml")
		secretPath                    = exutil.FixturePath("testdata", "openshift-secret-to-jenkins-credential.yaml")
		successfulPipeline            = exutil.FixturePath("testdata", "builds", "build-pruning", "successful-pipeline.yaml")
		failedPipeline                = exutil.FixturePath("testdata", "builds", "build-pruning", "failed-pipeline.yaml")
		pollingInterval               = time.Second
		timeout                       = time.Minute
		oc                            = exutil.NewCLI("jenkins-pipeline", exutil.KubeConfigPath())
		ticker                        *time.Ticker
		j                             *jenkins.JenkinsRef
		pvs                           = []*corev1.PersistentVolume{}
		nfspod                        = &corev1.Pod{}

		cleanup = func(jenkinsTemplatePath string) {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}

			client := oc.AsAdmin().KubeFramework().ClientSet
			g.By("removing jenkins")
			exutil.RemoveDeploymentConfigs(oc, "jenkins")

			// per k8s e2e volume_util.go:VolumeTestCleanup, nuke any client pods
			// before nfs server to assist with umount issues; as such, need to clean
			// up prior to the AfterEach processing, to guaranteed deletion order
			if jenkinsTemplatePath == jenkinsPersistentTemplatePath {
				g.By("deleting PVCs")
				exutil.DeletePVCsForDeployment(client, oc, "jenkins")
				g.By("removing nfs pvs")
				for _, pv := range pvs {
					e2e.DeletePersistentVolume(client, pv.Name)
				}
				g.By("removing nfs pod")
				e2e.DeletePodWithWait(oc.AsAdmin().KubeFramework(), client, nfspod)
			}
		}
		setupJenkins = func(jenkinsTemplatePath string) {
			exutil.DumpDockerInfo()
			// Deploy Jenkins
			// NOTE, we use these tests for both a) nightly regression runs against the latest openshift jenkins image on docker hub, and
			// b) PR testing for changes to the various openshift jenkins plugins we support.  With scenario b), a docker image that extends
			// our jenkins image is built, where the proposed plugin change is injected, overwritting the current released version of the plugin.
			// Our test/PR jobs on ci.openshift create those images, as well as set env vars this test suite looks for.  When both the env var
			// and test image is present, a new image stream is created using the test image, and our jenkins template is instantiated with
			// an override to use that images stream and test image
			var licensePrefix, pluginName string
			useSnapshotImage := false

			err := oc.Run("create").Args("-n", oc.Namespace(), "-f", jenkinsTemplatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			jenkinsTemplateName := "jenkins-ephemeral"

			// create persistent volumes if running persistent jenkins
			if jenkinsTemplatePath == jenkinsPersistentTemplatePath {
				g.By("PV/PVC dump before setup")
				exutil.DumpPersistentVolumeInfo(oc)

				jenkinsTemplateName = "jenkins-persistent"

				nfspod, pvs, err = exutil.SetupK8SNFSServerAndVolume(oc, 3)
				o.Expect(err).NotTo(o.HaveOccurred())

			}

			// our pipeline jobs, between jenkins and oc invocations, need more mem than the default
			newAppArgs := []string{"--template", fmt.Sprintf("%s/%s", oc.Namespace(), jenkinsTemplateName), "-p", "MEMORY_LIMIT=2Gi", "-p", "DISABLE_ADMINISTRATIVE_MONITORS=true"}
			newAppArgs = jenkins.OverridePodTemplateImages(newAppArgs)
			clientPluginNewAppArgs, useClientPluginSnapshotImage := jenkins.SetupSnapshotImage(jenkins.UseLocalClientPluginSnapshotEnvVarName, localClientPluginSnapshotImage, localClientPluginSnapshotImageStream, newAppArgs, oc)
			syncPluginNewAppArgs, useSyncPluginSnapshotImage := jenkins.SetupSnapshotImage(jenkins.UseLocalSyncPluginSnapshotEnvVarName, localSyncPluginSnapshotImage, localSyncPluginSnapshotImageStream, newAppArgs, oc)

			switch {
			case useClientPluginSnapshotImage && useSyncPluginSnapshotImage:
				fmt.Fprintf(g.GinkgoWriter,
					"\nBOTH %s and %s for PR TESTING ARE SET.  WILL NOT CHOOSE BETWEEN THE TWO SO TESTING CURRENT PLUGIN VERSIONS IN LATEST OPENSHIFT JENKINS IMAGE ON DOCKER HUB.\n",
					jenkins.UseLocalClientPluginSnapshotEnvVarName, jenkins.UseLocalSyncPluginSnapshotEnvVarName)
			case useClientPluginSnapshotImage:
				fmt.Fprintf(g.GinkgoWriter, "\nTHE UPCOMING TESTS WILL LEVERAGE AN IMAGE THAT EXTENDS THE LATEST OPENSHIFT JENKINS IMAGE AND OVERRIDES THE OPENSHIFT CLIENT PLUGIN WITH A NEW VERSION BUILT FROM PROPOSED CHANGES TO THAT PLUGIN.\n")
				licensePrefix = clientLicenseText
				pluginName = clientPluginName
				useSnapshotImage = true
				newAppArgs = clientPluginNewAppArgs
			case useSyncPluginSnapshotImage:
				fmt.Fprintf(g.GinkgoWriter, "\nTHE UPCOMING TESTS WILL LEVERAGE AN IMAGE THAT EXTENDS THE LATEST OPENSHIFT JENKINS IMAGE AND OVERRIDES THE OPENSHIFT SYNC PLUGIN WITH A NEW VERSION BUILT FROM PROPOSED CHANGES TO THAT PLUGIN.\n")
				licensePrefix = syncLicenseText
				pluginName = syncPluginName
				useSnapshotImage = true
				newAppArgs = syncPluginNewAppArgs
			default:
				fmt.Fprintf(g.GinkgoWriter, "\nNO PR TEST ENV VARS SET SO TESTING CURRENT PLUGIN VERSIONS IN LATEST OPENSHIFT JENKINS IMAGE ON DOCKER HUB.\n")
			}

			g.By(fmt.Sprintf("calling oc new-app useSnapshotImage %v with license text %s and newAppArgs %#v", useSnapshotImage, licensePrefix, newAppArgs))
			err = oc.Run("new-app").Args(newAppArgs...).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			if jenkinsTemplatePath == jenkinsPersistentTemplatePath {
				g.By("PV/PVC dump after setup")
				exutil.DumpPersistentVolumeInfo(oc)
			}

			g.By("waiting for jenkins deployment")
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), "jenkins", 1, false, oc)
			if err != nil {
				exutil.DumpApplicationPodLogs("jenkins", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			j = jenkins.NewRef(oc)

			g.By("wait for jenkins to come up")
			_, err = j.WaitForContent("", 200, 10*time.Minute, "")

			if err != nil {
				exutil.DumpApplicationPodLogs("jenkins", oc)
			}

			o.Expect(err).NotTo(o.HaveOccurred())

			if useSnapshotImage {
				g.By("verifying the test image is being used")
				// for the test image, confirm that a snapshot version of the plugin is running in the jenkins image we'll test against
				_, err = j.WaitForContent(licensePrefix+` ([0-9\.]+)-SNAPSHOT`, 200, 10*time.Minute, "/pluginManager/plugin/"+pluginName+"/thirdPartyLicenses")
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			// Start capturing logs from this deployment config.
			// This command will terminate if the Jenkins instance crashes. This
			// ensures that even if the Jenkins DC restarts, we should capture
			// logs from the crash.
			_, _, _, err = oc.Run("logs").Args("-f", "dc/jenkins").Background()
			o.Expect(err).NotTo(o.HaveOccurred())

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for default service account")
			err = exutil.WaitForServiceAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()), "default")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("waiting for builder service account")
			err = exutil.WaitForServiceAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()), "builder")
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		debugAnyJenkinsFailure = func(br *exutil.BuildResult, name string, oc *exutil.CLI, dumpMaster bool) {
			if !br.BuildSuccess {
				br.LogDumper = jenkins.DumpLogs
				fmt.Fprintf(g.GinkgoWriter, "\n\n START debugAnyJenkinsFailure\n\n")
				j := jenkins.NewRef(oc)
				jobLog, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "")
				if err == nil {
					fmt.Fprintf(g.GinkgoWriter, "\n %s job log:\n%s", name, jobLog)
				} else {
					fmt.Fprintf(g.GinkgoWriter, "\n error getting %s job log: %#v", name, err)
				}
				if dumpMaster {
					exutil.DumpApplicationPodLogs("jenkins", oc)
				}
				fmt.Fprintf(g.GinkgoWriter, "\n\n END debugAnyJenkinsFailure\n\n")
			}
		}
	)

	g.Context("Sync plugin tests", func() {

		g.It("using the ephemeral template", func() {
			defer cleanup(jenkinsEphemeralTemplatePath)
			setupJenkins(jenkinsEphemeralTemplatePath)

			g.By("Deleted pipeline strategy buildconfigs")

			g.By("should not be recreated by the sync plugin", func() {
				// create the bc
				g.By(fmt.Sprintf("calling oc new-app -f %q", origPipelinePath))
				err := oc.Run("new-app").Args("-f", origPipelinePath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verify job is in jenkins")
				_, err = j.WaitForContent("", 200, 30*time.Second, "job/%s/job/%s-sample-pipeline/", oc.Namespace(), oc.Namespace())
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("delete pipeline strategy bc %q", origPipelinePath))
				err = oc.Run("delete").Args("bc", "sample-pipeline").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verify job is not in jenkins")
				_, err = j.WaitForContent("", 404, 30*time.Second, "job/%s/job/%s-sample-pipeline/", oc.Namespace(), oc.Namespace())
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verify bc is still deleted")
				err = oc.Run("get").Args("bc", "sample-pipeline").Execute()
				o.Expect(err).To(o.HaveOccurred())

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("all", "-l", "app=jenkins-pipeline-example").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Sync secret to credential")

			g.By("should map openshift secret to a jenkins credential as the secret is manipulated", func() {
				g.By("create secret for jenkins credential")
				err := oc.Run("create").Args("-f", secretPath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verify credential created since label should be there")
				// NOTE, for the credential URL in Jenkins
				// it returns rc 200 with no output if credential exists and a 404 if it does not exists
				_, err = j.WaitForContent("", 200, 10*time.Second, "credentials/store/system/domain/_/credential/%s-%s/", oc.Namespace(), secretName)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verify credential deleted when label removed")
				err = oc.Run("label").Args("secret", secretName, secretCredentialSyncLabel+"-").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = j.WaitForContent("", 404, 10*time.Second, "credentials/store/system/domain/_/credential/%s-%s/", oc.Namespace(), secretName)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verify credential added when label added")
				err = oc.Run("label").Args("secret", secretName, secretCredentialSyncLabel+"=true").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = j.WaitForContent("", 200, 10*time.Second, "credentials/store/system/domain/_/credential/%s-%s/", oc.Namespace(), secretName)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verify credential deleted when secret deleted")
				err = oc.Run("delete").Args("secret", secretName).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = j.WaitForContent("", 404, 10*time.Second, "credentials/store/system/domain/_/credential/%s-%s/", oc.Namespace(), secretName)
				o.Expect(err).NotTo(o.HaveOccurred())

				// no need to clean up, last operation above deleted the secret
			})

			//TODO - for these config map slave tests, as well and the imagestream/imagestreamtag
			// tests ... rather than actually running the pipelines, we could just inspect the config in jenkins
			// to make sure the k8s pod templates are there.
			// In general, while we want at least one verification somewhere in pipeline.go that the agent
			// images work, we should minimize the actually running of pipelines using them to only one
			// for each maven/nodejs
			g.By("Pipeline using config map slave")

			g.By("should build and complete successfully", func() {
				g.By("create the pod template with config map")
				err := oc.Run("create").Args("-f", configMapPodTemplatePath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("calling oc new-app -f %q", podTemplateSlavePipelinePath))
				err = oc.Run("new-app").Args("-f", podTemplateSlavePipelinePath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting the pipeline build and waiting for it to complete")
				// this just does sh "mvn --version"
				br, err := exutil.StartBuildAndWait(oc, "openshift-jee-sample")
				if err != nil || !br.BuildSuccess {
					exutil.DumpBuilds(oc)
				}
				debugAnyJenkinsFailure(br, oc.Namespace()+"-openshift-jee-sample", oc, true)
				br.AssertSuccess()

				g.By("getting job log, make sure has success message")
				out, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("making sure job log ran with our config map slave pod template")
				o.Expect(out).To(o.ContainSubstring("Running on jenkins-slave"))

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("configmap", "jenkins-slave").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("bc", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Pipeline using imagestream slave")

			g.By("should build and complete successfully", func() {
				g.By("create the pod template with imagestream")
				err := oc.Run("create").Args("-f", imagestreamPodTemplatePath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("calling oc new-app -f %q", podTemplateSlavePipelinePath))
				err = oc.Run("new-app").Args("-f", podTemplateSlavePipelinePath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting the pipeline build and waiting for it to complete")
				// this just does sh "mvn --version"
				br, err := exutil.StartBuildAndWait(oc, "openshift-jee-sample")
				if err != nil || !br.BuildSuccess {
					exutil.DumpBuilds(oc)
				}
				debugAnyJenkinsFailure(br, oc.Namespace()+"-openshift-jee-sample", oc, true)
				br.AssertSuccess()

				g.By("getting job log, making sure job log has success message")
				out, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("making sure job log ran with our config map slave pod template")
				o.Expect(out).To(o.ContainSubstring("Running on jenkins-slave"))

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("is", "jenkins-slave").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("bc", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Pipeline using imagestreamtag slave")

			g.By("should build and complete successfully", func() {
				g.By("create the pod template with imagestream")
				err := oc.Run("create").Args("-f", imagestreamtagPodTemplatePath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("calling oc new-app -f %q", podTemplateSlavePipelinePath))
				err = oc.Run("new-app").Args("-f", podTemplateSlavePipelinePath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting the pipeline build and waiting for it to complete")
				// this just does sh "mvn --version"
				br, err := exutil.StartBuildAndWait(oc, "openshift-jee-sample")
				if err != nil || !br.BuildSuccess {
					exutil.DumpBuilds(oc)
				}
				debugAnyJenkinsFailure(br, oc.Namespace()+"-openshift-jee-sample", oc, true)
				br.AssertSuccess()

				g.By("getting job log, making sure job log has success message")
				out, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("making sure job log ran with our config map slave pod template")
				o.Expect(out).To(o.ContainSubstring("Running on slave-jenkins"))

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("is", "slave-jenkins").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("bc", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Pipeline with env vars")

			g.By("should build and complete successfully", func() {
				// instantiate the bc
				g.By(fmt.Sprintf("calling oc new-app -f %q", envVarsPipelinePath))
				err := oc.Run("new-app").Args("-f", envVarsPipelinePath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				// start the build
				g.By("starting the pipeline build, including env var, and waiting for it to complete")
				br, err := exutil.StartBuildAndWait(oc, "-e", "FOO2=BAR2", "sample-pipeline-withenvs")
				if err != nil || !br.BuildSuccess {
					exutil.DumpBuilds(oc)
				}
				debugAnyJenkinsFailure(br, oc.Namespace()+"-sample-pipeline-withenvs", oc, true)
				br.AssertSuccess()

				g.By("confirm all the log annotations are there")
				_, err = jenkins.ProcessLogURLAnnotations(oc, br)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("get build console logs and see if succeeded")
				out, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
				if err != nil {
					exutil.DumpApplicationPodLogs("jenkins", oc)
					exutil.DumpBuilds(oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("and see if env is set")
				if !strings.Contains(out, "FOO2 is BAR2") {
					exutil.DumpApplicationPodLogs("jenkins", oc)
					exutil.DumpBuilds(oc)
					o.Expect(out).To(o.ContainSubstring("FOO2 is BAR2"))
				}

				// start the nextbuild
				g.By("starting the pipeline build and waiting for it to complete")
				br, err = exutil.StartBuildAndWait(oc, "sample-pipeline-withenvs")
				if err != nil || !br.BuildSuccess {
					exutil.DumpApplicationPodLogs("jenkins", oc)
					exutil.DumpBuilds(oc)
				}
				debugAnyJenkinsFailure(br, oc.Namespace()+"-sample-pipeline-withenvs", oc, true)
				br.AssertSuccess()

				g.By("get build console logs and see if succeeded")
				out, err = j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
				if err != nil {
					exutil.DumpApplicationPodLogs("jenkins", oc)
					exutil.DumpBuilds(oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("and see if env FOO1 is set")
				if !strings.Contains(out, "FOO1 is BAR1") {
					exutil.DumpApplicationPodLogs("jenkins", oc)
					exutil.DumpBuilds(oc)
					o.Expect(out).To(o.ContainSubstring("FOO1 is BAR1"))
				}

				g.By("and see if env FOO2 is still not set")
				if !strings.Contains(out, "FOO2 is null") {
					exutil.DumpApplicationPodLogs("jenkins", oc)
					exutil.DumpBuilds(oc)
					o.Expect(out).To(o.ContainSubstring("FOO2 is null"))
				}

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("bc", "sample-pipeline-withenvs").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("delete jenkins job runs when the associated build is deleted")

			g.By("should prune pipeline builds based on the buildConfig settings", func() {

				g.By("creating successful test pipeline")
				err := oc.Run("create").Args("-f", successfulPipeline).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting four test builds")
				// builds only do sh 'exit 0'
				for i := 0; i < 4; i++ {
					br, _ := exutil.StartBuildAndWait(oc, "successful-pipeline")
					br.AssertSuccess()
				}

				buildConfig, err := oc.BuildClient().Build().BuildConfigs(oc.Namespace()).Get("successful-pipeline", metav1.GetOptions{})
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "%v", err)
				}

				var builds *buildv1.BuildList

				g.By("waiting up to one minute for pruning to complete")
				err = wait.PollImmediate(pollingInterval, timeout, func() (bool, error) {
					builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{LabelSelector: buildutil.BuildConfigSelector("successful-pipeline").String()})
					if err != nil {
						fmt.Fprintf(g.GinkgoWriter, "%v", err)
						return false, err
					}
					if int32(len(builds.Items)) == *buildConfig.Spec.SuccessfulBuildsHistoryLimit {
						fmt.Fprintf(g.GinkgoWriter, "%v builds exist, retrying...", len(builds.Items))
						return true, nil
					}
					return false, nil
				})

				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "%v", err)
				}

				passed := false
				if int32(len(builds.Items)) == 2 || int32(len(builds.Items)) == 3 {
					passed = true
				}
				o.Expect(passed).To(o.BeTrue(), "there should be 2-3 completed builds left after pruning, but instead there were %v", len(builds.Items))

				g.By("creating failed test pipeline")
				err = oc.Run("create").Args("-f", failedPipeline).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting four test builds")
				for i := 0; i < 4; i++ {
					br, _ := exutil.StartBuildAndWait(oc, "failed-pipeline")
					br.AssertFailure()
				}

				buildConfig, err = oc.BuildClient().Build().BuildConfigs(oc.Namespace()).Get("failed-pipeline", metav1.GetOptions{})
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "%v", err)
				}

				g.By("waiting up to one minute for pruning to complete")
				err = wait.PollImmediate(pollingInterval, timeout, func() (bool, error) {
					builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{LabelSelector: buildutil.BuildConfigSelector("successful-pipeline").String()})
					if err != nil {
						fmt.Fprintf(g.GinkgoWriter, "%v", err)
						return false, err
					}
					if int32(len(builds.Items)) == *buildConfig.Spec.FailedBuildsHistoryLimit {
						fmt.Fprintf(g.GinkgoWriter, "%v builds exist, retrying...", len(builds.Items))
						return true, nil
					}
					return false, nil
				})

				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "%v", err)
				}

				passed = false
				if int32(len(builds.Items)) == 2 || int32(len(builds.Items)) == 3 {
					passed = true
				}
				o.Expect(passed).To(o.BeTrue(), "there should be 2-3 completed builds left after pruning, but instead there were %v", len(builds.Items))

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("bc", "successful-pipeline").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("bc", "failed-pipeline").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

			})

		})

	})

})
