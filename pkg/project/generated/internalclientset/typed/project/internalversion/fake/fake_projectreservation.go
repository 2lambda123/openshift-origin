package fake

import (
	project "github.com/openshift/origin/pkg/project/apis/project"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeProjectReservations implements ProjectReservationInterface
type FakeProjectReservations struct {
	Fake *FakeProject
}

var projectreservationsResource = schema.GroupVersionResource{Group: "project.openshift.io", Version: "", Resource: "projectreservations"}

var projectreservationsKind = schema.GroupVersionKind{Group: "project.openshift.io", Version: "", Kind: "ProjectReservation"}

// Get takes name of the projectReservation, and returns the corresponding projectReservation object, and an error if there is any.
func (c *FakeProjectReservations) Get(name string, options v1.GetOptions) (result *project.ProjectReservation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(projectreservationsResource, name), &project.ProjectReservation{})
	if obj == nil {
		return nil, err
	}
	return obj.(*project.ProjectReservation), err
}

// List takes label and field selectors, and returns the list of ProjectReservations that match those selectors.
func (c *FakeProjectReservations) List(opts v1.ListOptions) (result *project.ProjectReservationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(projectreservationsResource, projectreservationsKind, opts), &project.ProjectReservationList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &project.ProjectReservationList{}
	for _, item := range obj.(*project.ProjectReservationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested projectReservations.
func (c *FakeProjectReservations) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(projectreservationsResource, opts))
}

// Create takes the representation of a projectReservation and creates it.  Returns the server's representation of the projectReservation, and an error, if there is any.
func (c *FakeProjectReservations) Create(projectReservation *project.ProjectReservation) (result *project.ProjectReservation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(projectreservationsResource, projectReservation), &project.ProjectReservation{})
	if obj == nil {
		return nil, err
	}
	return obj.(*project.ProjectReservation), err
}

// Update takes the representation of a projectReservation and updates it. Returns the server's representation of the projectReservation, and an error, if there is any.
func (c *FakeProjectReservations) Update(projectReservation *project.ProjectReservation) (result *project.ProjectReservation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(projectreservationsResource, projectReservation), &project.ProjectReservation{})
	if obj == nil {
		return nil, err
	}
	return obj.(*project.ProjectReservation), err
}

// Delete takes name of the projectReservation and deletes it. Returns an error if one occurs.
func (c *FakeProjectReservations) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(projectreservationsResource, name), &project.ProjectReservation{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeProjectReservations) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(projectreservationsResource, listOptions)

	_, err := c.Fake.Invokes(action, &project.ProjectReservationList{})
	return err
}

// Patch applies the patch and returns the patched projectReservation.
func (c *FakeProjectReservations) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *project.ProjectReservation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(projectreservationsResource, name, data, subresources...), &project.ProjectReservation{})
	if obj == nil {
		return nil, err
	}
	return obj.(*project.ProjectReservation), err
}
