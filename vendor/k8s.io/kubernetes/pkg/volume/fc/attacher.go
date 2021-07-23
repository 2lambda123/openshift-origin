/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"
)

type fcAttacher struct {
	host    volume.VolumeHost
	manager diskManager
}

var _ volume.Attacher = &fcAttacher{}

var _ volume.AttachableVolumePlugin = &fcPlugin{}

func (plugin *fcPlugin) NewAttacher() (volume.Attacher, error) {
	return &fcAttacher{
		host:    plugin.host,
		manager: &FCUtil{},
	}, nil
}

func (plugin *fcPlugin) GetDeviceMountRefs(deviceMountPath string) ([]string, error) {
	mounter := plugin.host.GetMounter(plugin.GetPluginName())
	return mount.GetMountRefs(mounter, deviceMountPath)
}

func (attacher *fcAttacher) Attach(spec *volume.Spec, nodeName types.NodeName) (string, error) {
	return "", nil
}

func (attacher *fcAttacher) VolumesAreAttached(specs []*volume.Spec, nodeName types.NodeName) (map[*volume.Spec]bool, error) {
	volumesAttachedCheck := make(map[*volume.Spec]bool)
	for _, spec := range specs {
		volumesAttachedCheck[spec] = true
	}

	return volumesAttachedCheck, nil
}

func (attacher *fcAttacher) WaitForAttach(spec *volume.Spec, devicePath string, _ *v1.Pod, timeout time.Duration) (string, error) {
	mounter, err := volumeSpecToMounter(spec, attacher.host)
	if err != nil {
		glog.Warningf("failed to get fc mounter: %v", err)
		return "", err
	}
	return attacher.manager.AttachDisk(*mounter)
}

func (attacher *fcAttacher) GetDeviceMountPath(
	spec *volume.Spec) (string, error) {
	mounter, err := volumeSpecToMounter(spec, attacher.host)
	if err != nil {
		glog.Warningf("failed to get fc mounter: %v", err)
		return "", err
	}

	return attacher.manager.MakeGlobalPDName(*mounter.fcDisk), nil
}

func (attacher *fcAttacher) MountDevice(spec *volume.Spec, devicePath string, deviceMountPath string) error {
	mounter := attacher.host.GetMounter(fcPluginName)
	notMnt, err := mounter.IsLikelyNotMountPoint(deviceMountPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(deviceMountPath, 0750); err != nil {
				return err
			}
			notMnt = true
		} else {
			return err
		}
	}

	volumeSource, readOnly, err := getVolumeSource(spec)
	if err != nil {
		return err
	}

	options := []string{}
	if readOnly {
		options = append(options, "ro")
	}
	if notMnt {
		diskMounter := &mount.SafeFormatAndMount{Interface: mounter, Exec: attacher.host.GetExec(fcPluginName)}
		mountOptions := volumeutil.MountOptionFromSpec(spec, options...)
		err = diskMounter.FormatAndMount(devicePath, deviceMountPath, volumeSource.FSType, mountOptions)
		if err != nil {
			os.Remove(deviceMountPath)
			return err
		}
	}
	return nil
}

type fcDetacher struct {
	mounter mount.Interface
	manager diskManager
	host    volume.VolumeHost
}

var _ volume.Detacher = &fcDetacher{}

func (plugin *fcPlugin) NewDetacher() (volume.Detacher, error) {
	return &fcDetacher{
		mounter: plugin.host.GetMounter(plugin.GetPluginName()),
		manager: &FCUtil{},
		host:    plugin.host,
	}, nil
}

func (detacher *fcDetacher) Detach(volumeName string, nodeName types.NodeName) error {
	return nil
}

func (detacher *fcDetacher) UnmountDevice(deviceMountPath string) error {
	// Specify device name for DetachDisk later
	devName, _, err := mount.GetDeviceNameFromMount(detacher.mounter, deviceMountPath)
	if err != nil {
		glog.Errorf("fc: failed to get device from mnt: %s\nError: %v", deviceMountPath, err)
		return err
	}
	// Unmount for deviceMountPath(=globalPDPath)
	err = volumeutil.UnmountPath(deviceMountPath, detacher.mounter)
	if err != nil {
		return fmt.Errorf("fc: failed to unmount: %s\nError: %v", deviceMountPath, err)
	}
	unMounter := volumeSpecToUnmounter(detacher.mounter, detacher.host)
	// The device is unmounted now. If UnmountDevice was retried, GetDeviceNameFromMount
	// won't find any mount and won't return DetachDisk below.
	// Therefore implement our own retry mechanism here.
	// E.g. DetachDisk sometimes fails to flush a multipath device with "device is busy" when it was
	// just unmounted.
	// 2 minutes should be enough within 6 minute force detach timeout.
	var detachError error
	err = wait.PollImmediate(10*time.Second, 2*time.Minute, func() (bool, error) {
		detachError = detacher.manager.DetachDisk(*unMounter, devName)
		if detachError != nil {
			glog.V(4).Infof("fc: failed to detach disk %s (%s): %v", devName, deviceMountPath, detachError)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("fc: failed to detach disk: %s\nError: %v", devName, detachError)
	}
	glog.V(2).Infof("fc: successfully detached disk: %s", devName)
	return nil
}

func volumeSpecToMounter(spec *volume.Spec, host volume.VolumeHost) (*fcDiskMounter, error) {
	fc, readOnly, err := getVolumeSource(spec)
	if err != nil {
		return nil, err
	}
	var lun string
	var wwids []string
	if fc.Lun != nil && len(fc.TargetWWNs) != 0 {
		lun = strconv.Itoa(int(*fc.Lun))
	} else if len(fc.WWIDs) != 0 {
		for _, wwid := range fc.WWIDs {
			wwids = append(wwids, strings.Replace(wwid, " ", "_", -1))
		}
	} else {
		return nil, fmt.Errorf("fc: no fc disk information found. failed to make a new mounter")
	}
	fcDisk := &fcDisk{
		plugin: &fcPlugin{
			host: host,
		},
		wwns:  fc.TargetWWNs,
		lun:   lun,
		wwids: wwids,
		io:    &osIOHandler{},
	}
	// TODO: remove feature gate check after no longer needed
	if utilfeature.DefaultFeatureGate.Enabled(features.BlockVolume) {
		volumeMode, err := volumeutil.GetVolumeMode(spec)
		if err != nil {
			return nil, err
		}
		glog.V(5).Infof("fc: volumeSpecToMounter volumeMode %s", volumeMode)
		return &fcDiskMounter{
			fcDisk:     fcDisk,
			fsType:     fc.FSType,
			volumeMode: volumeMode,
			readOnly:   readOnly,
			mounter:    volumeutil.NewSafeFormatAndMountFromHost(fcPluginName, host),
			deviceUtil: volumeutil.NewDeviceHandler(volumeutil.NewIOHandler()),
		}, nil
	}
	return &fcDiskMounter{
		fcDisk:     fcDisk,
		fsType:     fc.FSType,
		readOnly:   readOnly,
		mounter:    volumeutil.NewSafeFormatAndMountFromHost(fcPluginName, host),
		deviceUtil: volumeutil.NewDeviceHandler(volumeutil.NewIOHandler()),
	}, nil
}

func volumeSpecToUnmounter(mounter mount.Interface, host volume.VolumeHost) *fcDiskUnmounter {
	return &fcDiskUnmounter{
		fcDisk: &fcDisk{
			io: &osIOHandler{},
		},
		mounter:    mounter,
		deviceUtil: volumeutil.NewDeviceHandler(volumeutil.NewIOHandler()),
		exec:       host.GetExec(fcPluginName),
	}
}
