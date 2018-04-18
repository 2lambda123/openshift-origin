package daemon

import (
	"k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	appsinformers "k8s.io/client-go/informers/apps/v1beta1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	extensionsinformers "k8s.io/client-go/informers/extensions/v1beta1"
	clientset "k8s.io/client-go/kubernetes"
)

func NewNodeSelectorAwareDaemonSetsController(openshiftDefaultNodeSelectorString, kubeDefaultNodeSelectorString string, namepaceInformer coreinformers.NamespaceInformer, daemonSetInformer extensionsinformers.DaemonSetInformer, historyInformer appsinformers.ControllerRevisionInformer, podInformer coreinformers.PodInformer, nodeInformer coreinformers.NodeInformer, kubeClient clientset.Interface) (*DaemonSetsController, error) {
	controller, err := NewDaemonSetsController(daemonSetInformer, historyInformer, podInformer, nodeInformer, kubeClient)
	if err != nil {
		return controller, err
	}
	controller.namespaceLister = namepaceInformer.Lister()
	controller.namespaceStoreSynced = namepaceInformer.Informer().HasSynced
	controller.openshiftDefaultNodeSelectorString = openshiftDefaultNodeSelectorString
	if len(controller.openshiftDefaultNodeSelectorString) > 0 {
		controller.openshiftDefaultNodeSelector, err = labels.Parse(controller.openshiftDefaultNodeSelectorString)
		if err != nil {
			return nil, err
		}
	}
	controller.kubeDefaultNodeSelectorString = kubeDefaultNodeSelectorString
	if len(controller.kubeDefaultNodeSelectorString) > 0 {
		controller.kubeDefaultNodeSelector, err = labels.Parse(controller.kubeDefaultNodeSelectorString)
		if err != nil {
			return nil, err
		}
	}

	return controller, nil
}

func (dsc *DaemonSetsController) namespaceNodeSelectorMatches(node *v1.Node, ds *extensions.DaemonSet) (bool, error) {
	if dsc.namespaceLister == nil {
		return true, nil
	}

	// this is racy (different listers) and we get to choose which way to fail.  This should requeue.
	ns, err := dsc.namespaceLister.Get(ds.Namespace)
	if apierrors.IsNotFound(err) {
		return false, err
	}
	// if we had any error, default to the safe option of creating a pod for the node.
	if err != nil {
		utilruntime.HandleError(err)
		return true, nil
	}

	return dsc.nodeSelectorMatches(node, ns), nil
}

func (dsc *DaemonSetsController) nodeSelectorMatches(node *v1.Node, ns *v1.Namespace) bool {
	originNodeSelector, ok := ns.Annotations["openshift.io/node-selector"]
	switch {
	case ok:
		selector, err := labels.Parse(originNodeSelector)
		if err == nil {
			if !selector.Matches(labels.Set(node.Labels)) {
				return false
			}
		}
	case !ok && len(dsc.openshiftDefaultNodeSelectorString) > 0:
		if !dsc.openshiftDefaultNodeSelector.Matches(labels.Set(node.Labels)) {
			return false
		}
	}

	kubeNodeSelector, ok := ns.Annotations["scheduler.alpha.kubernetes.io/node-selector"]
	switch {
	case ok:
		selector, err := labels.Parse(kubeNodeSelector)
		if err == nil {
			if !selector.Matches(labels.Set(node.Labels)) {
				return false
			}
		}
	case !ok && len(dsc.kubeDefaultNodeSelectorString) > 0:
		if !dsc.kubeDefaultNodeSelector.Matches(labels.Set(node.Labels)) {
			return false
		}
	}

	return true
}
