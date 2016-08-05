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

package persistentvolume

import (
	"errors"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

var class1Parameters = map[string]string{
	"param1": "value1",
}
var class2Parameters = map[string]string{
	"param2": "value2",
}
var storageClasses = []*extensions.StorageClass{
	{
		TypeMeta: unversioned.TypeMeta{
			Kind: "StorageClass",
		},

		ObjectMeta: api.ObjectMeta{
			Name: "gold",
		},

		Provisioner: mockPluginName,
		Parameters:  class1Parameters,
	},
	{
		TypeMeta: unversioned.TypeMeta{
			Kind: "StorageClass",
		},
		ObjectMeta: api.ObjectMeta{
			Name: "silver",
		},
		Provisioner: mockPluginName,
		Parameters:  class2Parameters,
	},
}

// call to storageClass 1, returning an error
var provision1Error = provisionCall{
	ret:                errors.New("Moc provisioner error"),
	expectedParameters: class1Parameters,
}

// call to storageClass 1, returning a valid PV
var provision1Success = provisionCall{
	ret:                nil,
	expectedParameters: class1Parameters,
}

// call to storageClass 2, returning a valid PV
var provision2Success = provisionCall{
	ret:                nil,
	expectedParameters: class2Parameters,
}

// Test single call to syncVolume, expecting provisioning to happen.
// 1. Fill in the controller with initial data
// 2. Call the syncVolume *once*.
// 3. Compare resulting volumes with expected volumes.
func TestProvisionSync(t *testing.T) {
	tests := []controllerTest{
		{
			// Provision a volume (with the default class)
			"11-1 - successful provision with storage class 1",
			novolumes,
			newVolumeArray("pvc-uid11-1", "1Gi", "uid11-1", "claim11-1", api.VolumeBound, api.PersistentVolumeReclaimDelete, annBoundByController, annDynamicallyProvisioned, annClass),
			newClaimArray("claim11-1", "uid11-1", "1Gi", "", api.ClaimPending, annClass),
			// Binding will be completed in the next syncClaim
			newClaimArray("claim11-1", "uid11-1", "1Gi", "", api.ClaimPending, annClass),
			noevents, noerrors, wrapTestWithProvisionCalls([]provisionCall{provision1Success}, testSyncClaim),
		},
		{
			// Provision failure - plugin not found
			"11-2 - plugin not found",
			novolumes,
			novolumes,
			newClaimArray("claim11-2", "uid11-2", "1Gi", "", api.ClaimPending, annClass),
			newClaimArray("claim11-2", "uid11-2", "1Gi", "", api.ClaimPending, annClass),
			[]string{"Warning ProvisioningFailed"}, noerrors,
			testSyncClaim,
		},
		{
			// Provision failure - newProvisioner returns error
			"11-3 - newProvisioner failure",
			novolumes,
			novolumes,
			newClaimArray("claim11-3", "uid11-3", "1Gi", "", api.ClaimPending, annClass),
			newClaimArray("claim11-3", "uid11-3", "1Gi", "", api.ClaimPending, annClass),
			[]string{"Warning ProvisioningFailed"}, noerrors,
			wrapTestWithProvisionCalls([]provisionCall{}, testSyncClaim),
		},
		{
			// Provision failure - Provision returns error
			"11-4 - provision failure",
			novolumes,
			novolumes,
			newClaimArray("claim11-4", "uid11-4", "1Gi", "", api.ClaimPending, annClass),
			newClaimArray("claim11-4", "uid11-4", "1Gi", "", api.ClaimPending, annClass),
			[]string{"Warning ProvisioningFailed"}, noerrors,
			wrapTestWithProvisionCalls([]provisionCall{provision1Error}, testSyncClaim),
		},
		{
			// No provisioning if there is a matching volume available
			"11-6 - provisioning when there is a volume available",
			newVolumeArray("volume11-6", "1Gi", "", "", api.VolumePending, api.PersistentVolumeReclaimRetain, annClass),
			newVolumeArray("volume11-6", "1Gi", "uid11-6", "claim11-6", api.VolumeBound, api.PersistentVolumeReclaimRetain, annBoundByController, annClass),
			newClaimArray("claim11-6", "uid11-6", "1Gi", "", api.ClaimPending, annClass),
			newClaimArray("claim11-6", "uid11-6", "1Gi", "volume11-6", api.ClaimBound, annClass, annBoundByController, annBindCompleted),
			noevents, noerrors,
			// No provisioning plugin confingure - makes the test fail when
			// the controller errorneously tries to provision something
			wrapTestWithProvisionCalls([]provisionCall{provision1Success}, testSyncClaim),
		},
		{
			// Provision success? - claim is bound before provisioner creates
			// a volume.
			"11-7 - claim is bound before provisioning",
			novolumes,
			newVolumeArray("pvc-uid11-7", "1Gi", "uid11-7", "claim11-7", api.VolumeBound, api.PersistentVolumeReclaimDelete, annBoundByController, annDynamicallyProvisioned, annClass),
			newClaimArray("claim11-7", "uid11-7", "1Gi", "", api.ClaimPending, annClass),
			// The claim would be bound in next syncClaim
			newClaimArray("claim11-7", "uid11-7", "1Gi", "", api.ClaimPending, annClass),
			noevents, noerrors,
			wrapTestWithInjectedOperation(wrapTestWithProvisionCalls([]provisionCall{}, testSyncClaim), func(ctrl *PersistentVolumeController, reactor *volumeReactor) {
				// Create a volume before provisionClaimOperation starts.
				// This similates a parallel controller provisioning the volume.
				reactor.lock.Lock()
				volume := newVolume("pvc-uid11-7", "1Gi", "uid11-7", "claim11-7", api.VolumeBound, api.PersistentVolumeReclaimDelete, annBoundByController, annDynamicallyProvisioned, annClass)
				reactor.volumes[volume.Name] = volume
				reactor.lock.Unlock()
			}),
		},
		{
			// Provision success - cannot save provisioned PV once,
			// second retry succeeds
			"11-8 - cannot save provisioned volume",
			novolumes,
			newVolumeArray("pvc-uid11-8", "1Gi", "uid11-8", "claim11-8", api.VolumeBound, api.PersistentVolumeReclaimDelete, annBoundByController, annDynamicallyProvisioned, annClass),
			newClaimArray("claim11-8", "uid11-8", "1Gi", "", api.ClaimPending, annClass),
			// Binding will be completed in the next syncClaim
			newClaimArray("claim11-8", "uid11-8", "1Gi", "", api.ClaimPending, annClass),
			noevents,
			[]reactorError{
				// Inject error to the first
				// kubeclient.PersistentVolumes.Create() call. All other calls
				// will succeed.
				{"create", "persistentvolumes", errors.New("Mock creation error")},
			},
			wrapTestWithProvisionCalls([]provisionCall{provision1Success}, testSyncClaim),
		},
		{
			// Provision success? - cannot save provisioned PV five times,
			// volume is deleted and delete succeeds
			"11-9 - cannot save provisioned volume, delete succeeds",
			novolumes,
			novolumes,
			newClaimArray("claim11-9", "uid11-9", "1Gi", "", api.ClaimPending, annClass),
			newClaimArray("claim11-9", "uid11-9", "1Gi", "", api.ClaimPending, annClass),
			[]string{"Warning ProvisioningFailed"},
			[]reactorError{
				// Inject error to five kubeclient.PersistentVolumes.Create()
				// calls
				{"create", "persistentvolumes", errors.New("Mock creation error1")},
				{"create", "persistentvolumes", errors.New("Mock creation error2")},
				{"create", "persistentvolumes", errors.New("Mock creation error3")},
				{"create", "persistentvolumes", errors.New("Mock creation error4")},
				{"create", "persistentvolumes", errors.New("Mock creation error5")},
			},
			wrapTestWithPluginCalls(
				nil,                                // recycle calls
				[]error{nil},                       // delete calls
				[]provisionCall{provision1Success}, // provision calls
				testSyncClaim,
			),
		},
		{
			// Provision failure - cannot save provisioned PV five times,
			// volume delete failed - no plugin found
			"11-10 - cannot save provisioned volume, no delete plugin found",
			novolumes,
			novolumes,
			newClaimArray("claim11-10", "uid11-10", "1Gi", "", api.ClaimPending, annClass),
			newClaimArray("claim11-10", "uid11-10", "1Gi", "", api.ClaimPending, annClass),
			[]string{"Warning ProvisioningFailed", "Warning ProvisioningCleanupFailed"},
			[]reactorError{
				// Inject error to five kubeclient.PersistentVolumes.Create()
				// calls
				{"create", "persistentvolumes", errors.New("Mock creation error1")},
				{"create", "persistentvolumes", errors.New("Mock creation error2")},
				{"create", "persistentvolumes", errors.New("Mock creation error3")},
				{"create", "persistentvolumes", errors.New("Mock creation error4")},
				{"create", "persistentvolumes", errors.New("Mock creation error5")},
			},
			// No deleteCalls are configured, which results into no deleter plugin available for the volume
			wrapTestWithProvisionCalls([]provisionCall{provision1Success}, testSyncClaim),
		},
		{
			// Provision failure - cannot save provisioned PV five times,
			// volume delete failed - deleter returns error five times
			"11-11 - cannot save provisioned volume, deleter fails",
			novolumes,
			novolumes,
			newClaimArray("claim11-11", "uid11-11", "1Gi", "", api.ClaimPending, annClass),
			newClaimArray("claim11-11", "uid11-11", "1Gi", "", api.ClaimPending, annClass),
			[]string{"Warning ProvisioningFailed", "Warning ProvisioningCleanupFailed"},
			[]reactorError{
				// Inject error to five kubeclient.PersistentVolumes.Create()
				// calls
				{"create", "persistentvolumes", errors.New("Mock creation error1")},
				{"create", "persistentvolumes", errors.New("Mock creation error2")},
				{"create", "persistentvolumes", errors.New("Mock creation error3")},
				{"create", "persistentvolumes", errors.New("Mock creation error4")},
				{"create", "persistentvolumes", errors.New("Mock creation error5")},
			},
			wrapTestWithPluginCalls(
				nil, // recycle calls
				[]error{ // delete calls
					errors.New("Mock deletion error1"),
					errors.New("Mock deletion error2"),
					errors.New("Mock deletion error3"),
					errors.New("Mock deletion error4"),
					errors.New("Mock deletion error5"),
				},
				[]provisionCall{provision1Success}, // provision calls
				testSyncClaim),
		},
		{
			// Provision failure - cannot save provisioned PV five times,
			// volume delete succeeds 2nd time
			"11-12 - cannot save provisioned volume, delete succeeds 2nd time",
			novolumes,
			novolumes,
			newClaimArray("claim11-12", "uid11-12", "1Gi", "", api.ClaimPending, annClass),
			newClaimArray("claim11-12", "uid11-12", "1Gi", "", api.ClaimPending, annClass),
			[]string{"Warning ProvisioningFailed"},
			[]reactorError{
				// Inject error to five kubeclient.PersistentVolumes.Create()
				// calls
				{"create", "persistentvolumes", errors.New("Mock creation error1")},
				{"create", "persistentvolumes", errors.New("Mock creation error2")},
				{"create", "persistentvolumes", errors.New("Mock creation error3")},
				{"create", "persistentvolumes", errors.New("Mock creation error4")},
				{"create", "persistentvolumes", errors.New("Mock creation error5")},
			},
			wrapTestWithPluginCalls(
				nil, // recycle calls
				[]error{ // delete calls
					errors.New("Mock deletion error1"),
					nil,
				}, //  provison calls
				[]provisionCall{provision1Success},
				testSyncClaim,
			),
		},
		{
			// Provision a volume (with non-default class)
			"11-13 - successful provision with storage class 2",
			novolumes,
			volumeWithClass("silver", newVolumeArray("pvc-uid11-13", "1Gi", "uid11-13", "claim11-13", api.VolumeBound, api.PersistentVolumeReclaimDelete, annBoundByController, annDynamicallyProvisioned)),
			claimWithClass("silver", newClaimArray("claim11-13", "uid11-13", "1Gi", "", api.ClaimPending)),
			// Binding will be completed in the next syncClaim
			claimWithClass("silver", newClaimArray("claim11-13", "uid11-13", "1Gi", "", api.ClaimPending)),
			noevents, noerrors, wrapTestWithProvisionCalls([]provisionCall{provision2Success}, testSyncClaim),
		},
		{
			// Provision error - non existing class
			"11-14 - fail due to non-existing class",
			novolumes,
			novolumes,
			claimWithClass("non-existing", newClaimArray("claim11-14", "uid11-14", "1Gi", "", api.ClaimPending)),
			claimWithClass("non-existing", newClaimArray("claim11-14", "uid11-14", "1Gi", "", api.ClaimPending)),
			noevents, noerrors, wrapTestWithProvisionCalls([]provisionCall{}, testSyncClaim),
		},
	}
	runSyncTests(t, tests, storageClasses, storageClasses[0].Name)
}

// Test multiple calls to syncClaim/syncVolume and periodic sync of all
// volume/claims. The test follows this pattern:
// 0. Load the controller with initial data.
// 1. Call controllerTest.testCall() once as in TestSync()
// 2. For all volumes/claims changed by previous syncVolume/syncClaim calls,
//    call appropriate syncVolume/syncClaim (simulating "volume/claim changed"
//    events). Go to 2. if these calls change anything.
// 3. When all changes are processed and no new changes were made, call
//    syncVolume/syncClaim on all volumes/claims (simulating "periodic sync").
// 4. If some changes were done by step 3., go to 2. (simulation of
//    "volume/claim updated" events, eventually performing step 3. again)
// 5. When 3. does not do any changes, finish the tests and compare final set
//    of volumes/claims with expected claims/volumes and report differences.
// Some limit of calls in enforced to prevent endless loops.
func TestProvisionMultiSync(t *testing.T) {
	tests := []controllerTest{
		{
			// Provision a volume with binding
			"12-1 - successful provision",
			novolumes,
			newVolumeArray("pvc-uid12-1", "1Gi", "uid12-1", "claim12-1", api.VolumeBound, api.PersistentVolumeReclaimDelete, annBoundByController, annDynamicallyProvisioned, annClass),
			newClaimArray("claim12-1", "uid12-1", "1Gi", "", api.ClaimPending, annClass),
			// Binding will be completed in the next syncClaim
			newClaimArray("claim12-1", "uid12-1", "1Gi", "pvc-uid12-1", api.ClaimBound, annClass, annBoundByController, annBindCompleted),
			noevents, noerrors, wrapTestWithProvisionCalls([]provisionCall{provision1Success}, testSyncClaim),
		},
	}

	runMultisyncTests(t, tests, storageClasses, storageClasses[0].Name)
}

// When provisioning is disabled, provisioning a claim should instantly return nil
func TestDisablingDynamicProvisioner(t *testing.T) {
	ctrl := newTestController(nil, nil, nil, nil, false, "")
	retVal := ctrl.provisionClaim(nil)
	if retVal != nil {
		t.Errorf("Expected nil return but got %v", retVal)
	}
}
