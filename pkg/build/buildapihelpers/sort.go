package buildapihelpers

import buildinternalapi "github.com/openshift/origin/pkg/build/apis/build"

// BuildSliceByCreationTimestamp implements sort.Interface for []Build
// based on the CreationTimestamp field.
type BuildSliceByCreationTimestamp []buildinternalapi.Build

func (b BuildSliceByCreationTimestamp) Len() int {
	return len(b)
}

func (b BuildSliceByCreationTimestamp) Less(i, j int) bool {
	return b[i].CreationTimestamp.Before(&b[j].CreationTimestamp)
}

func (b BuildSliceByCreationTimestamp) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
