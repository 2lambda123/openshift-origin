package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

// FakeClusterResourceQuotas implements ClusterResourceQuotaInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeClusterResourceQuotas struct {
	Fake *Fake
}

var clusteResourceQuotasResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusterresourcequotas"}

func (c *FakeClusterResourceQuotas) Get(name string, options metav1.GetOptions) (*quotaapi.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(clusteResourceQuotasResource, name), &quotaapi.ClusterResourceQuota{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) List(opts metav1.ListOptions) (*quotaapi.ClusterResourceQuotaList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(clusteResourceQuotasResource, optsv1), &quotaapi.ClusterResourceQuotaList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuotaList), err
}

func (c *FakeClusterResourceQuotas) Create(inObj *quotaapi.ClusterResourceQuota) (*quotaapi.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(clusteResourceQuotasResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err
}

func (c *FakeClusterResourceQuotas) Update(inObj *quotaapi.ClusterResourceQuota) (*quotaapi.ClusterResourceQuota, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(clusteResourceQuotasResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err
}
func (c *FakeClusterResourceQuotas) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(clusteResourceQuotasResource, name), &quotaapi.ClusterResourceQuota{})
	return err
}

func (c *FakeClusterResourceQuotas) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(clusteResourceQuotasResource, optsv1))
}

func (c *FakeClusterResourceQuotas) UpdateStatus(inObj *quotaapi.ClusterResourceQuota) (*quotaapi.ClusterResourceQuota, error) {
	action := clientgotesting.UpdateActionImpl{}
	action.Verb = "update"
	action.Resource = clusteResourceQuotasResource
	action.Subresource = "status"
	action.Object = inObj

	obj, err := c.Fake.Invokes(action, inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*quotaapi.ClusterResourceQuota), err

}
