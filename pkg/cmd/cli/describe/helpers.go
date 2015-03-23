package describe

import (
	"bytes"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

const emptyString = "<none>"

func tabbedString(f func(*tabwriter.Writer) error) (string, error) {
	out := new(tabwriter.Writer)
	b := make([]byte, 1024)
	buf := bytes.NewBuffer(b)
	out.Init(buf, 0, 8, 1, '\t', 0)

	err := f(out)
	if err != nil {
		return "", err
	}

	out.Flush()
	str := string(buf.String())
	return str, nil
}

func toString(v interface{}) string {
	value := fmt.Sprintf("%s", v)
	if len(value) == 0 {
		value = emptyString
	}
	return value
}

func bold(v interface{}) string {
	return "\033[1m" + toString(v) + "\033[0m"
}

func convertEnv(env []api.EnvVar) map[string]string {
	result := make(map[string]string, len(env))
	for _, e := range env {
		result[e.Name] = toString(e.Value)
	}
	return result
}

func formatString(out *tabwriter.Writer, label string, v interface{}) {
	fmt.Fprintf(out, fmt.Sprintf("%s:\t%s\n", label, toString(v)))
}

func formatImageRepositoryTags(out *tabwriter.Writer, label string, tags map[string]string) {
	fmt.Fprint(out, label)
	if len(tags) == 0 {
		fmt.Fprintf(out, "\t<none>\n")
		return
	}

	sortedTags := []string{}
	for k := range tags {
		sortedTags = append(sortedTags, k)
	}
	sort.Strings(sortedTags)
	for _, tag := range sortedTags {
		image := tags[tag]
		fmt.Fprintf(out, "\t%s=%s\n", tag, image)
	}
}

func formatLabels(labelMap map[string]string) string {
	return labels.Set(labelMap).String()
}

func extractAnnotations(annotations map[string]string, keys ...string) ([]string, map[string]string) {
	extracted := make([]string, len(keys))
	remaining := make(map[string]string)
	for k, v := range annotations {
		remaining[k] = v
	}
	for i, key := range keys {
		extracted[i] = remaining[key]
		delete(remaining, key)
	}
	return extracted, remaining
}

func formatAnnotations(out *tabwriter.Writer, m api.ObjectMeta, prefix string) {
	values, annotations := extractAnnotations(m.Annotations, "description")
	if len(values[0]) > 0 {
		formatString(out, prefix+"Description", values[0])
	}
	if len(annotations) > 0 {
		formatString(out, prefix+"Annotations", formatLabels(annotations))
	}
}

func formatMeta(out *tabwriter.Writer, m api.ObjectMeta) {
	formatString(out, "Name", m.Name)
	if !m.CreationTimestamp.IsZero() {
		formatString(out, "Created", m.CreationTimestamp)
	}
	formatString(out, "Labels", formatLabels(m.Labels))
	formatAnnotations(out, m, "")
}

// webhookURL assembles map with of webhook type as key and webhook url and value
func webhookURL(c *buildapi.BuildConfig, configHost string) map[string]string {
	result := map[string]string{}
	for i, trigger := range c.Triggers {
		whTrigger := ""
		switch trigger.Type {
		case "github":
			whTrigger = trigger.GithubWebHook.Secret
		case "generic":
			whTrigger = trigger.GenericWebHook.Secret
		}
		if len(whTrigger) == 0 {
			continue
		}
		apiVersion := latest.Version
		host := "localhost"
		if len(configHost) > 0 {
			host = configHost
		}
		url := fmt.Sprintf("%s/osapi/%s/buildConfigHooks/%s/%s/%s",
			host,
			apiVersion,
			c.Name,
			whTrigger,
			c.Triggers[i].Type,
		)
		result[string(trigger.Type)] = url
	}
	return result
}
