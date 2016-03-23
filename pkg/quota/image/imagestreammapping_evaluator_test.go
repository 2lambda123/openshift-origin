package image

import (
	"fmt"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kquota "k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagetest "github.com/openshift/origin/pkg/quota/image/testutil"
)

func TestImageStreamMappingEvaluatorUsage(t *testing.T) {
	for _, tc := range []struct {
		name             string
		iss              []imageapi.ImageStream
		imageName        string
		imageManifest    string
		imageAnnotations map[string]string
		destISNamespace  string
		destISName       string
		expectedImages   int64
	}{
		{
			name: "empty image stream",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "is",
					},
					Status: imageapi.ImageStreamStatus{},
				},
			},
			imageName:        imagetest.MiscImageDigest,
			imageManifest:    imagetest.MiscImage,
			imageAnnotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
			destISNamespace:  "test",
			destISName:       "is",
			expectedImages:   1,
		},

		{
			name:             "no image stream",
			imageName:        imagetest.MiscImageDigest,
			imageManifest:    imagetest.MiscImage,
			imageAnnotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
			destISNamespace:  "test",
			destISName:       "is",
			expectedImages:   1,
		},

		{
			name: "missing image annotation",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "is",
					},
					Status: imageapi.ImageStreamStatus{},
				},
			},
			imageName:       imagetest.MiscImageDigest,
			imageManifest:   imagetest.MiscImage,
			destISNamespace: "test",
			destISName:      "is",
			expectedImages:  0,
		},

		{
			name: "update existing tag",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "havingtag",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/havingtag@%s", imagetest.BaseImageWith1LayerDigest),
										Image:                imagetest.BaseImageWith1LayerDigest,
									},
								},
							},
						},
					},
				},
			},
			imageName:        imagetest.ChildImageWith2LayersDigest,
			imageManifest:    imagetest.ChildImageWith2Layers,
			imageAnnotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
			destISNamespace:  "test",
			destISName:       "havingtag",
			expectedImages:   1,
		},

		{
			name: "add a new tag with 2 image streams",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "destis",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/destis@%s", imagetest.BaseImageWith1LayerDigest),
										Image:                imagetest.BaseImageWith1LayerDigest,
									},
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", imagetest.MiscImageDigest),
										Image:                imagetest.MiscImageDigest,
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "other",
						Name:      "is2",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", imagetest.BaseImageWith2LayersDigest),
										Image:                imagetest.BaseImageWith2LayersDigest,
									},
								},
							},
						},
					},
				},
			},
			imageName:        imagetest.ChildImageWith3LayersDigest,
			imageManifest:    imagetest.ChildImageWith3Layers,
			imageAnnotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
			destISNamespace:  "test",
			destISName:       "destis",
			expectedImages:   1,
		},

		{
			name: "add a new tag to a new image stream with image present in the other",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "other",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", imagetest.BaseImageWith2LayersDigest),
										Image:                imagetest.BaseImageWith2LayersDigest,
									},
								},
							},
						},
					},
				},
			},
			imageName:        imagetest.BaseImageWith2LayersDigest,
			imageManifest:    imagetest.BaseImageWith2Layers,
			imageAnnotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
			destISNamespace:  "test",
			destISName:       "destis",
			expectedImages:   0,
		},
	} {

		fakeClient := &testclient.Fake{}
		fakeClient.AddReactor("list", "imagestreams", imagetest.GetFakeImageStreamListHandler(t, tc.iss...))
		fakeClient.AddReactor("get", "imagestreamimages", imagetest.GetFakeImageStreamImageGetHandler(t, "test", tc.iss...))

		evaluator := NewImageStreamMappingEvaluator(fakeClient)

		ism := &imageapi.ImageStreamMapping{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: tc.destISNamespace,
				Name:      tc.destISName,
			},
			Image: imageapi.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name:        tc.imageName,
					Annotations: tc.imageAnnotations,
				},
				DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s@%s", tc.destISNamespace, tc.destISName, tc.imageName),
				DockerImageManifest:  tc.imageManifest,
			},
		}

		usage := evaluator.Usage(ism)

		if len(usage) != len(expectedResources) {
			t.Errorf("[%s]: got unexpected number of computed resources: %d != %d", tc.name, len(usage), len(expectedResources))
		}

		masked := kquota.Mask(usage, expectedResources)
		expectedUsage := kapi.ResourceList{
			imageapi.ResourceImages: *resource.NewQuantity(tc.expectedImages, resource.DecimalSI),
		}

		if len(masked) != len(expectedUsage) {
			for k := range usage {
				if _, exists := masked[k]; !exists {
					t.Errorf("[%s]: got unexpected resource %q from Usage() method", tc.name, k)
				}
			}

			for k := range expectedUsage {
				if _, exists := masked[k]; !exists {
					t.Errorf("[%s]: expected resource %q not computed", tc.name, k)
				}
			}
		}

		for rname, expectedValue := range expectedUsage {
			if v, exists := masked[rname]; exists {
				if v.Cmp(expectedValue) != 0 {
					t.Errorf("[%s]: got unexpected usage for %q: %s != %s", tc.name, rname, v.String(), expectedValue.String())
				}
			}
		}
	}
}
