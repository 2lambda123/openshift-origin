package describe

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/gonum/graph"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	"github.com/openshift/origin/pkg/api/graph/graphview"
	kubeedges "github.com/openshift/origin/pkg/api/kubegraph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildedges "github.com/openshift/origin/pkg/build/graph"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployedges "github.com/openshift/origin/pkg/deploy/graph"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageedges "github.com/openshift/origin/pkg/image/graph"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

// ProjectStatusDescriber generates extended information about a Project
type ProjectStatusDescriber struct {
	K kclient.Interface
	C client.Interface
}

func (d *ProjectStatusDescriber) MakeGraph(namespace string) (osgraph.Graph, error) {
	g := osgraph.New()

	svcs, err := d.K.Services(namespace).List(labels.Everything())
	if err != nil {
		return g, err
	}

	iss, err := d.C.ImageStreams(namespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return g, err
	}

	bcs, err := d.C.BuildConfigs(namespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return g, err
	}

	dcs, err := d.C.DeploymentConfigs(namespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return g, err
	}

	builds := &buildapi.BuildList{}
	if len(bcs.Items) > 0 {
		if b, err := d.C.Builds(namespace).List(labels.Everything(), fields.Everything()); err == nil {
			builds = b
		}
	}

	rcs, err := d.K.ReplicationControllers(namespace).List(labels.Everything())
	if err != nil {
		rcs = &kapi.ReplicationControllerList{}
	}

	for i := range iss.Items {
		imagegraph.EnsureImageStreamNode(g, &iss.Items[i])
		imagegraph.EnsureAllImageStreamTagNodes(g, &iss.Items[i])
	}
	for i := range bcs.Items {
		build := buildgraph.EnsureBuildConfigNode(g, &bcs.Items[i])
		buildedges.AddInputOutputEdges(g, build)
		buildedges.JoinBuilds(build, builds.Items)
	}
	for i := range dcs.Items {
		deploy := deploygraph.EnsureDeploymentConfigNode(g, &dcs.Items[i])
		deployedges.AddTriggerEdges(g, deploy)
		deployedges.JoinDeployments(deploy, rcs.Items)
	}
	for i := range svcs.Items {
		service := kubegraph.EnsureServiceNode(g, &svcs.Items[i])
		kubeedges.AddExposedPodTemplateSpecEdges(g, service)
	}

	imageedges.AddAllImageStreamRefEdges(g)

	return g, nil
}

// Describe returns the description of a project
func (d *ProjectStatusDescriber) Describe(namespace, name string) (string, error) {
	g, err := d.MakeGraph(namespace)
	if err != nil {
		return "", err
	}

	project, err := d.C.Projects().Get(namespace)
	if err != nil {
		return "", err
	}

	coveredNodes := graphview.IntSet{}

	services, coveredByServices := graphview.AllServiceGroups(g, coveredNodes)
	coveredNodes.Insert(coveredByServices.List()...)

	standaloneDCs, coveredByDCs := graphview.AllDeploymentConfigPipelines(g, coveredNodes)
	coveredNodes.Insert(coveredByDCs.List()...)

	standaloneBCs, coveredByBCs := graphview.AllImagePipelinesFromBuildConfig(g, coveredNodes)
	coveredNodes.Insert(coveredByBCs.List()...)

	return tabbedString(func(out *tabwriter.Writer) error {
		indent := "  "
		fmt.Fprintf(out, "In project %s\n", projectapi.DisplayNameAndNameForProject(project))

		for _, service := range services {
			fmt.Fprintln(out)
			printLines(out, indent, 0, describeServiceInServiceGroup(service)...)

			for _, dcPipeline := range service.DeploymentConfigPipelines {
				printLines(out, indent, 1, describeDeploymentInServiceGroup(dcPipeline)...)
			}
		}

		for _, standaloneDC := range standaloneDCs {
			fmt.Fprintln(out)
			printLines(out, indent, 0, describeDeploymentInServiceGroup(standaloneDC)...)
		}

		for _, standaloneBC := range standaloneBCs {
			fmt.Fprintln(out)
			printLines(out, indent, 0, describeStandaloneBuildGroup(standaloneBC, namespace)...)
			printLines(out, indent, 1, describeAdditionalBuildDetail(standaloneBC.Build, standaloneBC.DestinationResolved, true)...)
		}

		if (len(services) == 0) && (len(standaloneDCs) == 0) && (len(standaloneBCs) == 0) {
			fmt.Fprintln(out)
			fmt.Fprintln(out, "You have no services, deployment configs, or build configs.")
			fmt.Fprintln(out, "Run 'oc new-app' to create an application.")

		} else {
			fmt.Fprintln(out)

			if hasUnresolvedImageStreamTag(g) {
				fmt.Fprintln(out, "Warning: Some of your builds are pointing to image streams, but the administrator has not configured the integrated Docker registry (oadm registry).")

			}
			if syntheticNodes := g.SyntheticNodes(); len(syntheticNodes) > 0 {
				fmt.Fprintln(out, strings.Join(describeMissingResources(g, syntheticNodes), "\n"))
			}

			fmt.Fprintln(out, "To see more, use 'oc describe service <name>' or 'oc describe dc <name>'.")
			fmt.Fprintln(out, "You can use 'oc get all' to see a list of other objects.")
		}

		return nil
	})
}

func describeMissingResources(g osgraph.Graph, missingNodes []graph.Node) []string {
	ret := []string{"Warning: some references could not be resolved:"}

	for _, node := range missingNodes {
		predecessorNames := []string{}
		for _, predecessor := range g.Predecessors(node) {
			predecessorNames = append(predecessorNames, g.GraphDescriber.Name(predecessor))
		}

		successorNames := []string{}
		for _, successor := range g.Successors(node) {
			successorNames = append(successorNames, g.GraphDescriber.Name(successor))
		}

		switch {
		case len(predecessorNames) > 0 && len(successorNames) > 0:
			ret = append(ret, fmt.Sprintf("\t%s used by %s referencing %s was not found", g.GraphDescriber.Name(node), strings.Join(predecessorNames, ","), strings.Join(successorNames, ",")))
		case len(predecessorNames) > 0:
			ret = append(ret, fmt.Sprintf("\t%s used by %s was not found", g.GraphDescriber.Name(node), strings.Join(predecessorNames, ",")))
		case len(successorNames) > 0:
			ret = append(ret, fmt.Sprintf("\t%s referencing %s was not found", g.GraphDescriber.Name(node), strings.Join(successorNames, ",")))
		}
	}

	return ret
}

// hasUnresolvedImageStreamTag checks all build configs that will output to an IST backed by an ImageStream and checks to make sure their builds can push.
func hasUnresolvedImageStreamTag(g osgraph.Graph) bool {
	for _, bcNode := range g.NodesByKind(buildgraph.BuildConfigNodeKind) {
		for _, istNode := range g.SuccessorNodesByEdgeKind(bcNode, buildedges.BuildOutputEdgeKind) {
			for _, uncastImageStreamNode := range g.SuccessorNodesByEdgeKind(istNode, imageedges.ReferencedImageStreamGraphEdgeKind) {
				imageStreamNode := uncastImageStreamNode.(*imagegraph.ImageStreamNode)
				if len(imageStreamNode.Status.DockerImageRepository) == 0 {
					return true
				}
			}
		}
	}

	return false
}

func printLines(out io.Writer, indent string, depth int, lines ...string) {
	for i, s := range lines {
		fmt.Fprintf(out, strings.Repeat(indent, depth))
		if i != 0 {
			fmt.Fprint(out, indent)
		}
		fmt.Fprintln(out, s)
	}
}

func describeDeploymentInServiceGroup(deploy graphview.DeploymentConfigPipeline) []string {
	includeLastPass := deploy.Deployment.ActiveDeployment == nil
	if len(deploy.Images) == 1 {
		lines := []string{fmt.Sprintf("%s deploys %s %s", deploy.Deployment.Name, describeImageInPipeline(deploy.Images[0], deploy.Deployment.Namespace), describeDeploymentConfigTrigger(deploy.Deployment.DeploymentConfig))}
		if len(lines[0]) > 120 && strings.Contains(lines[0], " <- ") {
			segments := strings.SplitN(lines[0], " <- ", 2)
			lines[0] = segments[0] + " <-"
			lines = append(lines, segments[1])
		}
		lines = append(lines, describeAdditionalBuildDetail(deploy.Images[0].Build, deploy.Images[0].DestinationResolved, includeLastPass)...)
		lines = append(lines, describeDeployments(deploy.Deployment, 3)...)
		return lines
	}

	lines := []string{fmt.Sprintf("%s deploys: %s", deploy.Deployment.Name, describeDeploymentConfigTrigger(deploy.Deployment.DeploymentConfig))}
	for _, image := range deploy.Images {
		lines = append(lines, describeImageInPipeline(image, deploy.Deployment.Namespace))
		lines = append(lines, describeAdditionalBuildDetail(image.Build, image.DestinationResolved, includeLastPass)...)
		lines = append(lines, describeDeployments(deploy.Deployment, 3)...)
	}
	return lines
}

func describeDeploymentConfigTrigger(dc *deployapi.DeploymentConfig) string {
	if len(dc.Triggers) == 0 {
		return "(manual)"
	}

	return ""
}

func describeStandaloneBuildGroup(pipeline graphview.ImagePipeline, namespace string) []string {
	switch {
	case pipeline.Build != nil:
		lines := []string{fmt.Sprintf("%s %s", pipeline.Build.BuildConfig.Name, describeBuildInPipeline(pipeline.Build.BuildConfig, pipeline.BaseImage))}
		if pipeline.Image != nil {
			lines = append(lines, fmt.Sprintf("pushes to %s", describeImageTagInPipeline(pipeline.Image, namespace)))
		}
		return lines
	case pipeline.Image != nil:
		return []string{describeImageTagInPipeline(pipeline.Image, namespace)}
	default:
		return []string{"<unknown>"}
	}
}

func describeImageInPipeline(pipeline graphview.ImagePipeline, namespace string) string {
	switch {
	case pipeline.Image != nil && pipeline.Build != nil:
		return fmt.Sprintf("%s <- %s", describeImageTagInPipeline(pipeline.Image, namespace), describeBuildInPipeline(pipeline.Build.BuildConfig, pipeline.BaseImage))
	case pipeline.Image != nil:
		return describeImageTagInPipeline(pipeline.Image, namespace)
	case pipeline.Build != nil:
		return describeBuildInPipeline(pipeline.Build.BuildConfig, pipeline.BaseImage)
	default:
		return "<unknown>"
	}
}

func describeImageTagInPipeline(image graphview.ImageTagLocation, namespace string) string {
	switch t := image.(type) {
	case *imagegraph.ImageStreamTagNode:
		if t.ImageStreamTag.Namespace != namespace {
			return image.ImageSpec()
		}
		return t.ImageStreamTag.Name
	default:
		return image.ImageSpec()
	}
}

func describeBuildInPipeline(build *buildapi.BuildConfig, baseImage graphview.ImageTagLocation) string {
	switch build.Parameters.Strategy.Type {
	case buildapi.DockerBuildStrategyType:
		// TODO: handle case where no source repo
		source, ok := describeSourceInPipeline(&build.Parameters.Source)
		if !ok {
			return "docker build; no source set"
		}
		return fmt.Sprintf("docker build of %s", source)
	case buildapi.SourceBuildStrategyType:
		source, ok := describeSourceInPipeline(&build.Parameters.Source)
		if !ok {
			return fmt.Sprintf("unconfigured source build %s", build.Name)
		}
		if baseImage == nil {
			return fmt.Sprintf("%s; no image set", source)
		}
		return fmt.Sprintf("builds %s with %s", source, baseImage.ImageSpec())
	case buildapi.CustomBuildStrategyType:
		source, ok := describeSourceInPipeline(&build.Parameters.Source)
		if !ok {
			return fmt.Sprintf("custom build %s", build.Name)
		}
		return fmt.Sprintf("custom build of %s", source)
	default:
		return fmt.Sprintf("unrecognized build %s", build.Name)
	}
}

func describeAdditionalBuildDetail(build *buildgraph.BuildConfigNode, pushTargetResolved bool, includeSuccess bool) []string {
	if build == nil {
		return nil
	}
	out := []string{}

	pass := build.LastSuccessfulBuild
	passTime := buildTimestamp(pass)
	fail := build.LastUnsuccessfulBuild
	failTime := buildTimestamp(fail)

	last := failTime
	if passTime.After(failTime.Time) {
		last = passTime
		fail = nil
	}

	if pass != nil && includeSuccess {
		out = append(out, describeBuildStatus(pass, &passTime, build.BuildConfig.Name, pushTargetResolved))
	}
	if fail != nil {
		out = append(out, describeBuildStatus(fail, &failTime, build.BuildConfig.Name, pushTargetResolved))
	}

	active := build.ActiveBuilds
	if len(active) > 0 {
		activeOut := []string{}
		for i := range active {
			activeOut = append(activeOut, describeBuildStatus(&active[i], nil, build.BuildConfig.Name, pushTargetResolved))
		}

		if buildTimestamp(&active[0]).Before(last) {
			out = append(out, activeOut...)
		} else {
			out = append(activeOut, out...)
		}
	}
	if len(out) == 0 && pass == nil {
		out = append(out, "not built yet")
	}
	return out
}

func describeBuildStatus(build *buildapi.Build, t *util.Time, parentName string, pushTargetResolved bool) string {
	imageStreamFailure := ""
	// if we're using an image stream and that image stream is the internal registry and that registry doesn't exist
	if (build.Parameters.Output.To != nil) && !pushTargetResolved {
		imageStreamFailure = " (can't push to image)"
	}

	if t == nil {
		ts := buildTimestamp(build)
		t = &ts
	}
	var time string
	if t.IsZero() {
		time = "<unknown>"
	} else {
		time = strings.ToLower(formatRelativeTime(t.Time))
	}
	name := build.Name
	prefix := parentName + "-"
	if strings.HasPrefix(name, prefix) {
		name = name[len(prefix):]
	}
	revision := describeSourceRevision(build.Parameters.Revision)
	if len(revision) != 0 {
		revision = fmt.Sprintf(" - %s", revision)
	}
	switch build.Status {
	case buildapi.BuildStatusComplete:
		return fmt.Sprintf("build %s succeeded %s ago%s%s", name, time, revision, imageStreamFailure)
	case buildapi.BuildStatusError:
		return fmt.Sprintf("build %s stopped with an error %s ago%s%s", name, time, revision, imageStreamFailure)
	case buildapi.BuildStatusFailed:
		return fmt.Sprintf("build %s failed %s ago%s%s", name, time, revision, imageStreamFailure)
	default:
		status := strings.ToLower(string(build.Status))
		return fmt.Sprintf("build %s %s for %s%s%s", name, status, time, revision, imageStreamFailure)
	}
}

func describeSourceRevision(rev *buildapi.SourceRevision) string {
	if rev == nil {
		return ""
	}
	switch {
	case rev.Git != nil:
		author := describeSourceControlUser(rev.Git.Author)
		if len(author) == 0 {
			author = describeSourceControlUser(rev.Git.Committer)
		}
		if len(author) != 0 {
			author = fmt.Sprintf(" (%s)", author)
		}
		commit := rev.Git.Commit
		if len(commit) > 7 {
			commit = commit[:7]
		}
		return fmt.Sprintf("%s: %s%s", commit, rev.Git.Message, author)
	default:
		return ""
	}
}

func describeSourceControlUser(user buildapi.SourceControlUser) string {
	if len(user.Name) == 0 {
		return user.Email
	}
	if len(user.Email) == 0 {
		return user.Name
	}
	return fmt.Sprintf("%s <%s>", user.Name, user.Email)
}

func buildTimestamp(build *buildapi.Build) util.Time {
	if build == nil {
		return util.Time{}
	}
	if !build.CompletionTimestamp.IsZero() {
		return *build.CompletionTimestamp
	}
	if !build.StartTimestamp.IsZero() {
		return *build.StartTimestamp
	}
	return build.CreationTimestamp
}

func describeSourceInPipeline(source *buildapi.BuildSource) (string, bool) {
	switch source.Type {
	case buildapi.BuildSourceGit:
		if len(source.Git.Ref) == 0 {
			return source.Git.URI, true
		}
		return fmt.Sprintf("%s#%s", source.Git.URI, source.Git.Ref), true
	}
	return "", false
}

func describeDeployments(node *deploygraph.DeploymentConfigNode, count int) []string {
	if node == nil {
		return nil
	}
	out := []string{}
	deployments := node.Deployments

	if node.ActiveDeployment == nil {
		on, auto := describeDeploymentConfigTriggers(node.DeploymentConfig)
		if node.DeploymentConfig.LatestVersion == 0 {
			out = append(out, fmt.Sprintf("#1 deployment waiting %s", on))
		} else if auto {
			out = append(out, fmt.Sprintf("#%d deployment pending %s", node.DeploymentConfig.LatestVersion, on))
		}
		// TODO: detect new image available?
	} else {
		deployments = append([]*kapi.ReplicationController{node.ActiveDeployment}, deployments...)
	}

	for i, deployment := range deployments {
		out = append(out, describeDeploymentStatus(deployment, i == 0))

		switch {
		case count == -1:
			if deployutil.DeploymentStatusFor(deployment) == deployapi.DeploymentStatusComplete {
				return out
			}
		default:
			if i+1 >= count {
				return out
			}
		}
	}
	return out
}

func describeDeploymentStatus(deploy *kapi.ReplicationController, first bool) string {
	timeAt := strings.ToLower(formatRelativeTime(deploy.CreationTimestamp.Time))
	status := deployutil.DeploymentStatusFor(deploy)
	version := deployutil.DeploymentVersionFor(deploy)
	switch status {
	case deployapi.DeploymentStatusFailed:
		reason := deployutil.DeploymentStatusReasonFor(deploy)
		if len(reason) > 0 {
			reason = fmt.Sprintf(": %s", reason)
		}
		// TODO: encode fail time in the rc
		return fmt.Sprintf("#%d deployment failed %s ago%s%s", version, timeAt, reason, describeDeploymentPodSummaryInline(deploy, false))
	case deployapi.DeploymentStatusComplete:
		// TODO: pod status output
		return fmt.Sprintf("#%d deployed %s ago%s", version, timeAt, describeDeploymentPodSummaryInline(deploy, first))
	case deployapi.DeploymentStatusRunning:
		return fmt.Sprintf("#%d deployment running for %s%s", version, timeAt, describeDeploymentPodSummaryInline(deploy, false))
	default:
		return fmt.Sprintf("#%d deployment %s %s ago%s", version, strings.ToLower(string(status)), timeAt, describeDeploymentPodSummaryInline(deploy, false))
	}
}

func describeDeploymentPodSummaryInline(deploy *kapi.ReplicationController, includeEmpty bool) string {
	s := describeDeploymentPodSummary(deploy, includeEmpty)
	if len(s) == 0 {
		return s
	}
	change := ""
	desired := deploy.Spec.Replicas
	switch {
	case desired < deploy.Status.Replicas:
		change = fmt.Sprintf(" reducing to %d", desired)
	case desired > deploy.Status.Replicas:
		change = fmt.Sprintf(" growing to %d", desired)
	}
	return fmt.Sprintf(" - %s%s", s, change)
}

func describeDeploymentPodSummary(deploy *kapi.ReplicationController, includeEmpty bool) string {
	actual, requested := deploy.Status.Replicas, deploy.Spec.Replicas
	if actual == requested {
		switch {
		case actual == 0:
			if !includeEmpty {
				return ""
			}
			return "0 pods"
		case actual > 1:
			return fmt.Sprintf("%d pods", actual)
		default:
			return "1 pod"
		}
	}
	return fmt.Sprintf("%d/%d pods", actual, requested)
}

func describeDeploymentConfigTriggers(config *deployapi.DeploymentConfig) (string, bool) {
	hasConfig, hasImage := false, false
	for _, t := range config.Triggers {
		switch t.Type {
		case deployapi.DeploymentTriggerOnConfigChange:
			hasConfig = true
		case deployapi.DeploymentTriggerOnImageChange:
			hasImage = true
		}
	}
	switch {
	case hasConfig && hasImage:
		return "on image or update", true
	case hasConfig:
		return "on update", true
	case hasImage:
		return "on image", true
	default:
		return "for manual", false
	}
}

func describeServiceInServiceGroup(svc graphview.ServiceGroup) []string {
	spec := svc.Service.Spec
	ip := spec.ClusterIP
	port := describeServicePorts(spec)
	switch {
	case ip == "None":
		return []string{fmt.Sprintf("service %s (headless)%s", svc.Service.Name, port)}
	case len(ip) == 0:
		return []string{fmt.Sprintf("service %s <initializing>%s", svc.Service.Name, port)}
	default:
		return []string{fmt.Sprintf("service %s - %s%s", svc.Service.Name, ip, port)}
	}
}

func describeServicePorts(spec kapi.ServiceSpec) string {
	switch len(spec.Ports) {
	case 0:
		return " no ports"

	case 1:
		if spec.Ports[0].TargetPort.String() == "0" || spec.ClusterIP == kapi.ClusterIPNone || spec.Ports[0].Port == spec.Ports[0].TargetPort.IntVal {
			return fmt.Sprintf(":%d", spec.Ports[0].Port)
		}
		return fmt.Sprintf(":%d -> %s", spec.Ports[0].Port, spec.Ports[0].TargetPort.String())

	default:
		pairs := []string{}
		for _, port := range spec.Ports {
			if port.TargetPort.String() == "0" || spec.ClusterIP == kapi.ClusterIPNone {
				pairs = append(pairs, fmt.Sprintf("%d", port.Port))
				continue
			}
			if port.Port == port.TargetPort.IntVal {
				pairs = append(pairs, port.TargetPort.String())
			} else {
				pairs = append(pairs, fmt.Sprintf("%d->%s", port.Port, port.TargetPort.String()))
			}
		}
		return " ports " + strings.Join(pairs, ", ")
	}
}
