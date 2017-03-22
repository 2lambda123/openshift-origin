package buildclone

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	buildapi "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/install"
	"github.com/openshift/origin/pkg/build/generator"
)

func TestCreateClone(t *testing.T) {
	rest := CloneREST{&generator.BuildGenerator{Client: generator.Client{
		CreateBuildFunc: func(ctx apirequest.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
			return &buildapi.Build{}, nil
		},
	}}}

	_, err := rest.Create(apirequest.NewDefaultContext(), &buildapi.BuildRequest{ObjectMeta: metav1.ObjectMeta{Name: "name"}})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCreateCloneValidationError(t *testing.T) {
	rest := CloneREST{&generator.BuildGenerator{}}
	_, err := rest.Create(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil {
		t.Error("Expected object got none!")
	}
}
