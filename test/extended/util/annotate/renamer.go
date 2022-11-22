package annotate

import (
	"regexp"
	"sort"
	"strings"
)

func GenerateName(name, filepath string) string {
	//func (r *ginkgoTestRenamer) generateRename(name, parentName string, node types.TestNode) {
	//originalName := name
	r := newGenerator()
	combinedName := name
	for {
		count := 0
		for _, label := range r.allLabels {
			// never apply a sig label twice
			if strings.HasPrefix(label, "[sig-") && strings.Contains(combinedName, "[sig-") {
				continue
			}
			if strings.Contains(combinedName, label) {
				continue
			}

			var needsLabel bool
			for _, segment := range r.stringMatches[label] {
				needsLabel = strings.Contains(combinedName, segment)
				if needsLabel {
					break
				}
			}
			if !needsLabel {
				if re := r.matches[label]; re != nil {
					needsLabel = r.matches[label].MatchString(combinedName)
				}
			}

			if needsLabel {
				count++
				combinedName += " " + label
				name += " " + label
			}
		}
		// if we didn't modify the test name, we're done.
		// if we did modify it, we need to process the new name to see if it now matches
		// additional labels it didn't previously match, so keep looping.
		if count == 0 {
			break
		}
	}

	if !r.excludedTestsFilter.MatchString(combinedName) {
		isSerial := strings.Contains(combinedName, "[Serial]")
		isConformance := strings.Contains(combinedName, "[Conformance]")
		switch {
		case isSerial && isConformance:
			name += " [Suite:openshift/conformance/serial/minimal]"
		case isSerial:
			name += " [Suite:openshift/conformance/serial]"
		case isConformance:
			name += " [Suite:openshift/conformance/parallel/minimal]"
		default:
			name += " [Suite:openshift/conformance/parallel]"
		}
	}
	if isGoModulePath(filepath, "k8s.io/kubernetes", "test/e2e") {
		name += " [Suite:k8s]"
	}

	return name
}

func newGenerator() *ginkgoTestRenamer {
	var allLabels []string
	matches := make(map[string]*regexp.Regexp)
	stringMatches := make(map[string][]string)
	for label, items := range TestMaps {
		sort.Strings(items)
		allLabels = append(allLabels, label)
		var remain []string
		for _, item := range items {
			re := regexp.MustCompile(item)
			if p, ok := re.LiteralPrefix(); ok {
				stringMatches[label] = append(stringMatches[label], p)
			} else {
				remain = append(remain, item)
			}
		}
		if len(remain) > 0 {
			matches[label] = regexp.MustCompile(strings.Join(remain, `|`))
		}
	}
	sort.Strings(allLabels)
	excludedTestsFilter := regexp.MustCompile(strings.Join(ExcludedTests, `|`))
	return &ginkgoTestRenamer{
		allLabels:           allLabels,
		stringMatches:       stringMatches,
		matches:             matches,
		excludedTestsFilter: excludedTestsFilter,
		output:              make(map[string]string),
	}
}

type ginkgoTestRenamer struct {
	// keys defined in TestMaps in openshift-hack/e2e/annotate/rules.go
	allLabels []string
	// exact substrings to match to apply a particular label
	stringMatches map[string][]string
	// regular expressions to match to apply a particular label
	matches map[string]*regexp.Regexp
	// regular expression excluding permanently a set of tests
	// see ExcludedTests in openshift-hack/e2e/annotate/rules.go
	excludedTestsFilter *regexp.Regexp
	// output from the generateRename and also input for updateNodeText
	output map[string]string
	// map of unmatched test names
	missing map[string]struct{}
}
