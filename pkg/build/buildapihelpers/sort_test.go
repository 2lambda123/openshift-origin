package buildapihelpers

import (
	"sort"
	"testing"
	"time"

	buildinternalapi "github.com/openshift/origin/pkg/build/apis/build"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSortBuildSliceByCreationTimestamp(t *testing.T) {
	present := metav1.Now()
	past := metav1.NewTime(present.Add(-time.Minute))
	builds := []buildinternalapi.Build{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "present",
				CreationTimestamp: present,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "past",
				CreationTimestamp: past,
			},
		},
	}
	sort.Sort(BuildSliceByCreationTimestamp(builds))
	if [2]string{builds[0].Name, builds[1].Name} != [2]string{"past", "present"} {
		t.Errorf("Unexpected sort order")
	}
}
