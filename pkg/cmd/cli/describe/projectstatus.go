package describe

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/graph"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// ProjectStatusDescriber generates extended information about a Project
type ProjectStatusDescriber struct {
	K kclient.Interface
	C client.Interface
}

func (d *ProjectStatusDescriber) Describe(namespace, name string) (string, error) {
	project, err := d.C.Projects().Get(namespace)
	if err != nil {
		return "", err
	}

	svcs, err := d.K.Services(namespace).List(labels.Everything())
	if err != nil {
		return "", err
	}

	bcs, err := d.C.BuildConfigs(namespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return "", err
	}

	dcs, err := d.C.DeploymentConfigs(namespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return "", err
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

	g := graph.New()
	for i := range bcs.Items {
		build := graph.BuildConfig(g, &bcs.Items[i])
		graph.JoinBuilds(build.(*graph.BuildConfigNode), builds.Items)
	}
	for i := range dcs.Items {
		deploy := graph.DeploymentConfig(g, &dcs.Items[i])
		graph.JoinDeployments(deploy.(*graph.DeploymentConfigNode), rcs.Items)
	}
	for i := range svcs.Items {
		graph.Service(g, &svcs.Items[i])
	}
	groups := graph.ServiceAndDeploymentGroups(graph.CoverServices(g))

	return tabbedString(func(out *tabwriter.Writer) error {
		indent := "  "
		if len(project.DisplayName) > 0 && project.DisplayName != namespace {
			fmt.Fprintf(out, "In project %s (%s)\n", project.DisplayName, namespace)
		} else {
			fmt.Fprintf(out, "In project %s\n", namespace)
		}

		for _, group := range groups {
			if len(group.Builds) != 0 {
				for _, build := range group.Builds {
					printLines(out, indent, 0, describeImageInPipeline(build, namespace))
					printLines(out, indent, 1, describeAdditionalBuildDetail(build.Build, true)...)
				}
				continue
			}
			if len(group.Services) == 0 {
				for _, deploy := range group.Deployments {
					fmt.Fprintln(out)
					printLines(out, indent, 0, describeDeploymentInServiceGroup(deploy)...)
				}
				continue
			}
			fmt.Fprintln(out)
			for _, svc := range group.Services {
				printLines(out, indent, 0, describeServiceInServiceGroup(svc)...)
			}
			for _, deploy := range group.Deployments {
				printLines(out, indent, 1, describeDeploymentInServiceGroup(deploy)...)
			}
		}

		if len(groups) == 0 {
			fmt.Fprintln(out, "\nYou have no services, deployment configs, or build configs. 'osc new-app' can be used to create applications from scratch from existing Docker images and templates.")
		} else {
			fmt.Fprintln(out, "\nTo see more information about a service or deployment config, use 'osc describe service <name>' or 'osc describe dc <name>'.")
			fmt.Fprintln(out, "You can use 'osc get pods,svc,dc,bc,builds' to see lists of each of the types described above.")
		}

		return nil
	})
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

func describeDeploymentInServiceGroup(deploy graph.DeploymentFlow) []string {
	includeLastPass := deploy.Deployment.ActiveDeployment == nil
	if len(deploy.Images) == 1 {
		lines := []string{fmt.Sprintf("%s deploys %s", deploy.Deployment.Name, describeImageInPipeline(deploy.Images[0], deploy.Deployment.Namespace))}
		if len(lines[0]) > 120 && strings.Contains(lines[0], " <- ") {
			segments := strings.SplitN(lines[0], " <- ", 2)
			lines[0] = segments[0] + " <-"
			lines = append(lines, segments[1])
		}
		lines = append(lines, describeAdditionalBuildDetail(deploy.Images[0].Build, includeLastPass)...)
		lines = append(lines, describeDeployments(deploy.Deployment, 3)...)
		return lines
	}
	lines := []string{fmt.Sprintf("%s deploys:", deploy.Deployment.Name)}
	for _, image := range deploy.Images {
		lines = append(lines, describeImageInPipeline(image, deploy.Deployment.Namespace))
		lines = append(lines, describeAdditionalBuildDetail(image.Build, includeLastPass)...)
		lines = append(lines, describeDeployments(deploy.Deployment, 3)...)
	}
	return lines
}

func describeImageInPipeline(pipeline graph.ImagePipeline, namespace string) string {
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

func describeImageTagInPipeline(image graph.ImageTagLocation, namespace string) string {
	switch t := image.(type) {
	case *graph.ImageStreamTagNode:
		if t.ImageStream.Namespace != namespace {
			return image.ImageSpec()
		}
		return fmt.Sprintf("%s:%s", t.ImageStream.Name, image.ImageTag())
	default:
		return image.ImageSpec()
	}
}

func describeBuildInPipeline(build *buildapi.BuildConfig, baseImage graph.ImageTagLocation) string {
	switch build.Parameters.Strategy.Type {
	case buildapi.DockerBuildStrategyType:
		// TODO: handle case where no source repo
		source, ok := describeSourceInPipeline(&build.Parameters.Source)
		if !ok {
			return "docker build; no source set"
		}
		return fmt.Sprintf("docker build of %s", source)
	case buildapi.STIBuildStrategyType:
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

func describeAdditionalBuildDetail(build *graph.BuildConfigNode, includeSuccess bool) []string {
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
		out = append(out, describeBuildStatus(pass, &passTime, build.BuildConfig.Name))
	}
	if fail != nil {
		out = append(out, describeBuildStatus(fail, &failTime, build.BuildConfig.Name))
	}

	active := build.ActiveBuilds
	if len(active) > 0 {
		activeOut := []string{}
		for i := range active {
			activeOut = append(activeOut, describeBuildStatus(&active[i], nil, build.BuildConfig.Name))
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

func describeBuildStatus(build *buildapi.Build, t *util.Time, parentName string) string {
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
	switch build.Status {
	case buildapi.BuildStatusComplete:
		return fmt.Sprintf("build %s succeeded %s", name, time)
	case buildapi.BuildStatusError:
		return fmt.Sprintf("build %s stopped with an error %s", name, time)
	default:
		status := strings.ToLower(string(build.Status))
		return fmt.Sprintf("build %s %s for %s", name, status, time)
	}
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

func describeDeployments(node *graph.DeploymentConfigNode, count int) []string {
	if node == nil {
		return nil
	}
	out := []string{}

	if node.ActiveDeployment == nil {
		on, auto := describeDeploymentConfigTriggers(node.DeploymentConfig)
		if node.DeploymentConfig.LatestVersion == 0 {
			out = append(out, fmt.Sprintf("#1 deployment waiting %s", on))
		} else if auto {
			out = append(out, fmt.Sprintf("#%d deployment pending %s", node.DeploymentConfig.LatestVersion, on))
		}
		// TODO: detect new image available?
	} else {
		out = append(out, describeDeploymentStatus(node.ActiveDeployment))
		count--
	}
	for i, deployment := range node.Deployments {
		if i >= count {
			break
		}
		out = append(out, describeDeploymentStatus(deployment))
	}
	return out
}

func describeDeploymentStatus(deploy *kapi.ReplicationController) string {
	timeAt := strings.ToLower(formatRelativeTime(deploy.CreationTimestamp.Time))
	switch s := deploy.Annotations[deployapi.DeploymentStatusAnnotation]; deployapi.DeploymentStatus(s) {
	case deployapi.DeploymentStatusFailed:
		// TODO: encode fail time in the rc
		return fmt.Sprintf("#%s deployment failed %s ago", deploy.Annotations[deployapi.DeploymentVersionAnnotation], timeAt)
	case deployapi.DeploymentStatusComplete:
		// TODO: pod status output
		return fmt.Sprintf("#%s deployed %s ago", deploy.Annotations[deployapi.DeploymentVersionAnnotation], timeAt)
	default:
		return fmt.Sprintf("#%s deployment %s %s ago", deploy.Annotations[deployapi.DeploymentVersionAnnotation], strings.ToLower(s), timeAt)
	}
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

func describeServiceInServiceGroup(svc graph.ServiceReference) []string {
	spec := svc.Service.Spec
	ip := spec.PortalIP
	port := describeServicePorts(spec)
	switch {
	case ip == "None":
		return []string{fmt.Sprintf("service %s (headless%s)", svc.Service.Name, port)}
	case len(ip) == 0:
		return []string{fmt.Sprintf("service %s (<initializing>%s)", svc.Service.Name, port)}
	default:
		return []string{fmt.Sprintf("service %s (%s%s)", svc.Service.Name, ip, port)}
	}
}

func describeServicePorts(spec kapi.ServiceSpec) string {
	switch len(spec.Ports) {
	case 0:
		return " no ports"
	case 1:
		if spec.Ports[0].TargetPort.String() == "0" || spec.PortalIP == kapi.PortalIPNone {
			return fmt.Sprintf(":%d", spec.Ports[0].Port)
		}
		return fmt.Sprintf(":%d -> %s", spec.Ports[0].Port, spec.Ports[0].TargetPort.String())
	default:
		pairs := []string{}
		for _, port := range spec.Ports {
			if port.TargetPort.String() == "0" || spec.PortalIP == kapi.PortalIPNone {
				pairs = append(pairs, fmt.Sprintf("%d", port.Port))
				continue
			}
			pairs = append(pairs, fmt.Sprintf("%d->%s", port.Port, port.TargetPort.String()))
		}
		return " " + strings.Join(pairs, ", ")
	}
}
