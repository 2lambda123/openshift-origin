package multitenant

import (
	"encoding/json"
	"strconv"

	"github.com/golang/glog"

	"github.com/openshift/openshift-sdn/pkg/ovssubnet"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/types"
	utilexec "k8s.io/kubernetes/pkg/util/exec"
)

const (
	initCmd     = "init"
	setUpCmd    = "setup"
	tearDownCmd = "teardown"
	statusCmd   = "status"
)

type MultitenantPlugin struct {
	host          knetwork.Host
	OvsController *ovssubnet.OvsController
}

func GetKubeNetworkPlugin() knetwork.NetworkPlugin {
	return &MultitenantPlugin{}
}

func (plugin *MultitenantPlugin) getExecutable() string {
	return "openshift-ovs-multitenant"
}

func (plugin *MultitenantPlugin) getVnid(namespace string) (uint, error) {
	// get vnid for the namespace
	vnid, ok := plugin.OvsController.VNIDMap[namespace]
	if !ok {
		// vnid does not exist for this pod, set it to zero (or error?)
		vnid = 0
	}
	return vnid, nil
}

func (plugin *MultitenantPlugin) Init(host knetwork.Host) error {
	plugin.host = host
	return nil
}

func (plugin *MultitenantPlugin) Name() string {
	return NetworkPluginName()
}

func (plugin *MultitenantPlugin) SetUpPod(namespace string, name string, id kubeletTypes.DockerID) error {
	vnid, err := plugin.getVnid(namespace)
	if err != nil {
		return err
	}
	out, err := utilexec.New().Command(plugin.getExecutable(), setUpCmd, namespace, name, string(id), strconv.FormatUint(uint64(vnid), 10)).CombinedOutput()
	glog.V(5).Infof("SetUpPod 'multitenant' network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *MultitenantPlugin) TearDownPod(namespace string, name string, id kubeletTypes.DockerID) error {
	vnid, err := plugin.getVnid(namespace)
	out, err := utilexec.New().Command(plugin.getExecutable(), tearDownCmd, namespace, name, string(id), strconv.FormatUint(uint64(vnid), 10)).CombinedOutput()
	glog.V(5).Infof("TearDownPod 'multitenant' network plugin output: %s, %v", string(out), err)
	return err
}

func (plugin *MultitenantPlugin) Status(namespace string, name string, id kubeletTypes.DockerID) (*knetwork.PodNetworkStatus, error) {
	vnid, err := plugin.getVnid(namespace)
	if err != nil {
		return nil, err
	}
	out, err := utilexec.New().Command(plugin.getExecutable(), statusCmd, namespace, name, string(id), strconv.FormatUint(uint64(vnid), 10)).CombinedOutput()
	glog.V(5).Infof("PodNetworkStatus 'multitenant' network plugin output: %s, %v", string(out), err)
	if err != nil {
		return nil, err
	}
	var podNetworkStatus knetwork.PodNetworkStatus
	err = json.Unmarshal([]byte(out), &podNetworkStatus)
	if err != nil {
		return nil, err
	}
	return &podNetworkStatus, nil
}
