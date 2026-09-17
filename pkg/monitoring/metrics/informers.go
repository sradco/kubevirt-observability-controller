/*
This file is part of the KubeVirt project

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Copyright The KubeVirt Authors.
*/

package metrics

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	k8sv1 "k8s.io/api/core/v1"
	toolscache "k8s.io/client-go/tools/cache"
	clonev1 "kubevirt.io/api/clone/v1beta1"
	k6tv1 "kubevirt.io/api/core/v1"
	exportv1 "kubevirt.io/api/export/v1"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
	poolv1 "kubevirt.io/api/pool/v1beta1"
	snapshotv1 "kubevirt.io/api/snapshot/v1beta1"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type storeIndexer interface {
	GetStore() toolscache.Store
	GetIndexer() toolscache.Indexer
}

func SetupInformers(ctx context.Context, c ctrlcache.Cache) error {
	vm, err := informerFor(ctx, c, &k6tv1.VirtualMachine{})
	if err != nil {
		return fmt.Errorf("VM: %w", err)
	}

	vmi, err := informerFor(ctx, c, &k6tv1.VirtualMachineInstance{})
	if err != nil {
		return fmt.Errorf("VMI: %w", err)
	}

	pvc, err := informerFor(ctx, c, &k8sv1.PersistentVolumeClaim{})
	if err != nil {
		return fmt.Errorf("PVC: %w", err)
	}

	instancetype, err := informerFor(
		ctx, c, &instancetypev1beta1.VirtualMachineInstancetype{},
	)
	if err != nil {
		return fmt.Errorf("instancetype: %w", err)
	}

	clusterInstancetype, err := informerFor(
		ctx, c, &instancetypev1beta1.VirtualMachineClusterInstancetype{},
	)
	if err != nil {
		return fmt.Errorf("clusterInstancetype: %w", err)
	}

	preference, err := informerFor(
		ctx, c, &instancetypev1beta1.VirtualMachinePreference{},
	)
	if err != nil {
		return fmt.Errorf("preference: %w", err)
	}

	clusterPreference, err := informerFor(
		ctx, c, &instancetypev1beta1.VirtualMachineClusterPreference{},
	)
	if err != nil {
		return fmt.Errorf("clusterPreference: %w", err)
	}

	controllerRevision, err := informerFor(
		ctx, c, &appsv1.ControllerRevision{},
	)
	if err != nil {
		return fmt.Errorf("controllerRevision: %w", err)
	}

	vmim, err := informerFor(
		ctx, c, &k6tv1.VirtualMachineInstanceMigration{},
	)
	if err != nil {
		return fmt.Errorf("VMIM: %w", err)
	}

	if err := vmim.GetIndexer().AddIndexers(toolscache.Indexers{
		ByMigrationUIDIndex: func(obj any) ([]string, error) {
			vmim, ok := obj.(*k6tv1.VirtualMachineInstanceMigration)
			if !ok {
				return nil, nil
			}
			return []string{string(vmim.UID)}, nil
		},
	}); err != nil {
		return fmt.Errorf("VMIM index: %w", err)
	}

	pod, err := informerFor(ctx, c, &k8sv1.Pod{})
	if err != nil {
		return fmt.Errorf("pod: %w", err)
	}

	vmSnapshot, err := informerFor(ctx, c, &snapshotv1.VirtualMachineSnapshot{})
	if err != nil {
		return fmt.Errorf("VMSnapshot: %w", err)
	}

	vmRestore, err := informerFor(ctx, c, &snapshotv1.VirtualMachineRestore{})
	if err != nil {
		return fmt.Errorf("VMRestore: %w", err)
	}

	vmExport, err := informerFor(ctx, c, &exportv1.VirtualMachineExport{})
	if err != nil {
		return fmt.Errorf("VMExport: %w", err)
	}

	vmClone, err := informerFor(ctx, c, &clonev1.VirtualMachineClone{})
	if err != nil {
		return fmt.Errorf("VMClone: %w", err)
	}

	vmPool, err := informerFor(ctx, c, &poolv1.VirtualMachinePool{})
	if err != nil {
		return fmt.Errorf("VMPool: %w", err)
	}

	SetStores(
		&Stores{
			VM:                  vm.GetStore(),
			VMI:                 vmi.GetStore(),
			PVC:                 pvc.GetStore(),
			Instancetype:        instancetype.GetStore(),
			ClusterInstancetype: clusterInstancetype.GetStore(),
			Preference:          preference.GetStore(),
			ClusterPreference:   clusterPreference.GetStore(),
			ControllerRevision:  controllerRevision.GetStore(),
			VirtHandlerPod:      pod.GetStore(),
			VMSnapshot:          vmSnapshot.GetStore(),
			VMRestore:           vmRestore.GetStore(),
			VMExport:            vmExport.GetStore(),
			VMClone:             vmClone.GetStore(),
			VMPool:              vmPool.GetStore(),
		},
		&Indexers{
			VMIMigration: vmim.GetIndexer(),
			KVPod:        pod.GetIndexer(),
		},
	)

	return nil
}

func informerFor(
	ctx context.Context, c ctrlcache.Cache, obj client.Object,
) (storeIndexer, error) {
	inf, err := c.GetInformer(ctx, obj)
	if err != nil {
		return nil, err
	}

	si, ok := inf.(storeIndexer)
	if !ok {
		return nil, fmt.Errorf(
			"informer %T does not expose store/indexer", inf,
		)
	}

	return si, nil
}
