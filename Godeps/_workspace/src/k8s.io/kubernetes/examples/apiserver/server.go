/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package main

import (
	"github.com/golang/glog"
	"k8s.io/kubernetes/cmd/libs/go2idl/client-gen/testdata/apis/testgroup/v1"
	testgroupetcd "k8s.io/kubernetes/examples/apiserver/rest"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/apimachinery"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/genericapiserver"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"

	// Install the testgroup API
	_ "k8s.io/kubernetes/cmd/libs/go2idl/client-gen/testdata/apis/testgroup/install"
)

func newStorageDestinations(groupName string, groupMeta *apimachinery.GroupMeta) (*genericapiserver.StorageDestinations, error) {
	storageDestinations := genericapiserver.NewStorageDestinations()
	var storageConfig etcdstorage.EtcdConfig
	storageConfig.ServerList = []string{"http://127.0.0.1:4001"}
	storageConfig.Prefix = genericapiserver.DefaultEtcdPathPrefix
	storageConfig.Codec = groupMeta.Codec
	storageInterface, err := storageConfig.NewStorage()
	if err != nil {
		return nil, err
	}
	storageDestinations.AddAPIGroup(groupName, storageInterface)
	return &storageDestinations, nil
}

func main() {
	config := genericapiserver.Config{
		EnableIndex:    true,
		APIPrefix:      "/api",
		APIGroupPrefix: "/apis",
	}
	s := genericapiserver.New(&config)

	groupVersion := v1.SchemeGroupVersion
	groupName := groupVersion.Group
	groupMeta, err := registered.Group(groupName)
	if err != nil {
		glog.Fatalf("%v", err)
	}
	storageDestinations, err := newStorageDestinations(groupName, groupMeta)
	if err != nil {
		glog.Fatalf("Unable to init etcd: %v", err)
	}
	restStorageMap := map[string]rest.Storage{
		"testtypes": testgroupetcd.NewREST(storageDestinations.Get(groupName, "testtype"), s.StorageDecorator()),
	}
	apiGroupInfo := genericapiserver.APIGroupInfo{
		GroupMeta: *groupMeta,
		VersionedResourcesStorageMap: map[string]map[string]rest.Storage{
			groupVersion.Version: restStorageMap,
		},
	}
	if err := s.InstallAPIGroups([]genericapiserver.APIGroupInfo{apiGroupInfo}); err != nil {
		glog.Fatalf("Error in installing API: %v", err)
	}
	s.Run(genericapiserver.NewServerRunOptions())
}
