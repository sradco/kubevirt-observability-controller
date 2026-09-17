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

package metrics_test

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/kubevirt-observability-controller/test/lib"
	"github.com/kubevirt/kubevirt-observability-controller/test/monitoring/metrics"
)

const (
	inventoryVMName   = "e2e-inventory-vm"
	inventoryVMIName  = "e2e-inventory-vmi"
	inventoryVMIMName = "e2e-inventory-vmim"
	inventoryPoolName = "e2e-inventory-pool"

	kubevirtAPIGroup   = "kubevirt.io"
	virtualMachineKind = "VirtualMachine"

	inventoryMetricTimeout = 2 * time.Minute
	inventoryMetricPoll    = 10 * time.Second
)

var _ = Describe("Inventory metrics", func() {
	Context("with a halted VirtualMachine", Ordered, func() {
		BeforeAll(func() {
			Expect(lib.CreateHaltedTestVM(testVMINamespace, inventoryVMName)).To(Succeed())
		})

		AfterAll(func() {
			Expect(lib.DeleteNamespacedResource(
				"vm", inventoryVMName, testVMINamespace,
			)).To(Succeed())
		})

		It("should expose kubevirt_vmsnapshot_info and create-date and drop them on delete", func() {
			skipWithoutCRD("virtualmachinesnapshots.snapshot.kubevirt.io")

			snapshotName := "snapshot-" + inventoryVMName
			Expect(lib.KubectlApply(vmSnapshotManifest(
				snapshotName, inventoryVMName, testVMINamespace,
			))).To(Succeed())
			DeferCleanup(func() error {
				return lib.DeleteNamespacedResource(
					"vmsnapshot", snapshotName, testVMINamespace,
				)
			})

			uid := resourceUID("vmsnapshot", snapshotName)
			infoLabels := map[string]string{
				"namespace": testVMINamespace,
				"name":      snapshotName,
				"uid":       uid,
				"vm":        inventoryVMName,
			}
			dateLabels := map[string]string{
				"name":      snapshotName,
				"namespace": testVMINamespace,
			}
			waitForMetricPresent("kubevirt_vmsnapshot_info", infoLabels)
			waitForMetricValue(
				"kubevirt_vmsnapshot_create_date_timestamp_seconds",
				dateLabels,
				creationUnix("vmsnapshot", snapshotName),
			)

			Expect(lib.DeleteNamespacedResource(
				"vmsnapshot", snapshotName, testVMINamespace,
			)).To(Succeed())
			waitForMetricAbsent("kubevirt_vmsnapshot_info", infoLabels)
			waitForMetricAbsent(
				"kubevirt_vmsnapshot_create_date_timestamp_seconds", dateLabels,
			)
		})

		It("should expose kubevirt_vmrestore_info and drop the series on delete", func() {
			skipWithoutCRD("virtualmachinerestores.snapshot.kubevirt.io")

			restoreName := "restore-" + inventoryVMName
			snapshotName := "snapshot-" + inventoryVMName
			Expect(lib.KubectlApply(vmRestoreManifest(
				restoreName, inventoryVMName, snapshotName, testVMINamespace,
			))).To(Succeed())
			DeferCleanup(func() error {
				return lib.DeleteNamespacedResource(
					"vmrestore", restoreName, testVMINamespace,
				)
			})

			uid := resourceUID("vmrestore", restoreName)
			infoLabels := map[string]string{
				"namespace":     testVMINamespace,
				"name":          restoreName,
				"uid":           uid,
				"vm":            inventoryVMName,
				"snapshot_name": snapshotName,
			}
			waitForMetricPresent("kubevirt_vmrestore_info", infoLabels)

			waitForMetricPresent("kubevirt_vm_info", map[string]string{
				"namespace": testVMINamespace,
				"name":      inventoryVMName,
			})

			Expect(lib.DeleteNamespacedResource(
				"vmrestore", restoreName, testVMINamespace,
			)).To(Succeed())
			waitForMetricAbsent("kubevirt_vmrestore_info", infoLabels)
		})

		It("should expose kubevirt_vmexport_info and TTL and drop them on delete", func() {
			skipWithoutCRD("virtualmachineexports.export.kubevirt.io")

			exportName := "export-" + inventoryVMName
			Expect(lib.KubectlApply(vmExportManifest(
				exportName, inventoryVMName, testVMINamespace,
			))).To(Succeed())
			DeferCleanup(func() error {
				return lib.DeleteNamespacedResource(
					"vmexport", exportName, testVMINamespace,
				)
			})

			uid := resourceUID("vmexport", exportName)
			infoLabels := map[string]string{
				"namespace":   testVMINamespace,
				"name":        exportName,
				"uid":         uid,
				"vm":          inventoryVMName,
				"source":      inventoryVMName,
				"source_kind": virtualMachineKind,
			}
			ttlLabels := map[string]string{
				"name":      exportName,
				"namespace": testVMINamespace,
			}
			waitForMetricPresent("kubevirt_vmexport_info", infoLabels)

			var ttlRaw string
			Eventually(func() string {
				var err error
				ttlRaw, err = lib.Kubectl(
					"get", "vmexport", exportName,
					"-n", testVMINamespace,
					"-o", "jsonpath={.status.ttlExpirationTime}",
				)
				if err != nil {
					return ""
				}
				return ttlRaw
			}, inventoryMetricTimeout, inventoryMetricPoll).ShouldNot(
				BeEmpty(),
				"expected VirtualMachineExport status.ttlExpirationTime",
			)
			waitForMetricValue(
				"kubevirt_vmexport_ttl_expiration_timestamp_seconds",
				ttlLabels,
				unixSeconds(ttlRaw),
			)

			Expect(lib.DeleteNamespacedResource(
				"vmexport", exportName, testVMINamespace,
			)).To(Succeed())
			waitForMetricAbsent("kubevirt_vmexport_info", infoLabels)
			waitForMetricAbsent(
				"kubevirt_vmexport_ttl_expiration_timestamp_seconds", ttlLabels,
			)
		})

		It("should expose kubevirt_vmclone_info and create-date and drop them on delete", func() {
			skipWithoutCRD("virtualmachineclones.clone.kubevirt.io")

			cloneName := "clone-" + inventoryVMName
			targetName := inventoryVMName + "-clone-target"
			Expect(lib.KubectlApply(vmCloneManifest(
				cloneName, inventoryVMName, targetName, testVMINamespace,
			))).To(Succeed())
			DeferCleanup(func() error {
				return errors.Join(
					lib.DeleteNamespacedResource(
						"vmclone", cloneName, testVMINamespace,
					),
					lib.DeleteNamespacedResource(
						"vm", targetName, testVMINamespace,
					),
				)
			})

			uid := resourceUID("vmclone", cloneName)
			infoLabels := map[string]string{
				"namespace":   testVMINamespace,
				"name":        cloneName,
				"uid":         uid,
				"source":      inventoryVMName,
				"source_kind": virtualMachineKind,
				"target_vm":   targetName,
			}
			dateLabels := map[string]string{
				"name":      cloneName,
				"namespace": testVMINamespace,
			}
			waitForMetricPresent("kubevirt_vmclone_info", infoLabels)
			waitForMetricValue(
				"kubevirt_vmclone_create_date_timestamp_seconds",
				dateLabels,
				creationUnix("vmclone", cloneName),
			)

			Expect(lib.DeleteNamespacedResource(
				"vmclone", cloneName, testVMINamespace,
			)).To(Succeed())
			waitForMetricAbsent("kubevirt_vmclone_info", infoLabels)
			waitForMetricAbsent(
				"kubevirt_vmclone_create_date_timestamp_seconds", dateLabels,
			)
		})
	})

	It("should expose pool info and replica gauges and drop them on delete", func() {
		skipWithoutCRD("virtualmachinepools.pool.kubevirt.io")

		Expect(lib.KubectlApply(vmPoolManifest(
			inventoryPoolName, testVMINamespace,
		))).To(Succeed())
		DeferCleanup(func() error {
			return lib.DeleteNamespacedResource(
				"vmpool", inventoryPoolName, testVMINamespace,
			)
		})

		uid := resourceUID("vmpool", inventoryPoolName)
		infoLabels := map[string]string{
			"namespace": testVMINamespace,
			"name":      inventoryPoolName,
			"uid":       uid,
		}
		replicaLabels := map[string]string{
			"namespace": testVMINamespace,
			"name":      inventoryPoolName,
		}
		waitForMetricPresent("kubevirt_vmpool_info", infoLabels)
		waitForMetricValue("kubevirt_vmpool_desired_replicas", replicaLabels, 0)
		waitForMetricValue("kubevirt_vmpool_replicas", replicaLabels, 0)
		waitForMetricValue("kubevirt_vmpool_ready_replicas", replicaLabels, 0)
		waitForMetricValue("kubevirt_vmpool_paused", replicaLabels, 0)
		waitForMetricValue("kubevirt_vmpool_replica_failure", replicaLabels, 0)

		Expect(lib.DeleteNamespacedResource(
			"vmpool", inventoryPoolName, testVMINamespace,
		)).To(Succeed())
		waitForMetricAbsent("kubevirt_vmpool_info", infoLabels)
		waitForMetricAbsent("kubevirt_vmpool_desired_replicas", replicaLabels)
		waitForMetricAbsent("kubevirt_vmpool_paused", replicaLabels)
		waitForMetricAbsent("kubevirt_vmpool_replica_failure", replicaLabels)
	})

	It("should expose kubevirt_vmi_migration_info and drop the series on delete", func() {
		Expect(lib.KubectlApply(inventoryVMIManifest(
			inventoryVMIName, testVMINamespace,
		))).To(Succeed())
		DeferCleanup(func() error {
			return lib.DeleteNamespacedResource(
				"vmi", inventoryVMIName, testVMINamespace,
			)
		})

		Expect(lib.KubectlApply(vmimManifest(
			inventoryVMIMName, inventoryVMIName, testVMINamespace,
		))).To(Succeed())
		DeferCleanup(func() error {
			return lib.DeleteNamespacedResource(
				"vmim", inventoryVMIMName, testVMINamespace,
			)
		})

		uid := resourceUID("vmim", inventoryVMIMName)
		infoLabels := map[string]string{
			"namespace": testVMINamespace,
			"name":      inventoryVMIName,
			"uid":       uid,
			"trigger":   "user",
		}
		waitForMetricPresent("kubevirt_vmi_migration_info", infoLabels)

		Expect(lib.DeleteNamespacedResource(
			"vmim", inventoryVMIMName, testVMINamespace,
		)).To(Succeed())
		waitForMetricAbsent("kubevirt_vmi_migration_info", infoLabels)
	})
})

func skipWithoutCRD(crd string) {
	if !lib.HasCRD(crd) {
		Skip("CRD " + crd + " is not installed")
	}
}

func resourceUID(kind, name string) string {
	uid, err := lib.Kubectl(
		"get", kind, name, "-n", testVMINamespace,
		"-o", "jsonpath={.metadata.uid}",
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(uid).ToNot(BeEmpty(),
		"expected UID for %s %s/%s", kind, testVMINamespace, name)
	return uid
}

func creationUnix(kind, name string) float64 {
	raw, err := lib.Kubectl(
		"get", kind, name, "-n", testVMINamespace,
		"-o", "jsonpath={.metadata.creationTimestamp}",
	)
	Expect(err).ToNot(HaveOccurred())
	return unixSeconds(raw)
}

func unixSeconds(raw string) float64 {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	Expect(err).ToNot(HaveOccurred())
	return float64(parsed.Unix())
}

func scrapeMatching(name string, labels map[string]string) ([]metrics.MetricLine, error) {
	output, err := metrics.Scrape(kvNamespace, metricsServiceName)
	if err != nil {
		return nil, err
	}
	return metrics.FindMetricWithLabels(output, name, labels), nil
}

func waitForMetricPresent(name string, labels map[string]string) {
	Eventually(func(g Gomega) {
		found, err := scrapeMatching(name, labels)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).ToNot(BeEmpty(),
			"expected %s with labels %v", name, labels)
	}, inventoryMetricTimeout, inventoryMetricPoll).Should(Succeed())
}

func waitForMetricValue(name string, labels map[string]string, want float64) {
	Eventually(func(g Gomega) {
		found, err := scrapeMatching(name, labels)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).ToNot(BeEmpty(),
			"expected %s with labels %v", name, labels)
		got, parseErr := strconv.ParseFloat(found[0].Value, 64)
		g.Expect(parseErr).ToNot(HaveOccurred())
		g.Expect(got).To(Equal(want),
			"expected %s value %v, got %v", name, want, got)
	}, inventoryMetricTimeout, inventoryMetricPoll).Should(Succeed())
}

func waitForMetricAbsent(name string, labels map[string]string) {
	Eventually(func() (int, error) {
		found, err := scrapeMatching(name, labels)
		if err != nil {
			return 0, err
		}
		return len(found), nil
	}, inventoryMetricTimeout, inventoryMetricPoll).Should(Equal(0),
		"expected %s with labels %v to disappear", name, labels)
}

func vmSnapshotManifest(name, vmName, namespace string) string {
	return fmt.Sprintf(`apiVersion: snapshot.kubevirt.io/v1beta1
kind: VirtualMachineSnapshot
metadata:
  name: %s
  namespace: %s
spec:
  source:
    apiGroup: %s
    kind: %s
    name: %s`, name, namespace, kubevirtAPIGroup, virtualMachineKind, vmName)
}

func vmRestoreManifest(name, vmName, snapshotName, namespace string) string {
	return fmt.Sprintf(`apiVersion: snapshot.kubevirt.io/v1beta1
kind: VirtualMachineRestore
metadata:
  name: %s
  namespace: %s
spec:
  target:
    apiGroup: %s
    kind: %s
    name: %s
  virtualMachineSnapshotName: %s`,
		name, namespace, kubevirtAPIGroup, virtualMachineKind, vmName, snapshotName)
}

func vmExportManifest(name, vmName, namespace string) string {
	return fmt.Sprintf(`apiVersion: export.kubevirt.io/v1
kind: VirtualMachineExport
metadata:
  name: %s
  namespace: %s
spec:
  source:
    apiGroup: %s
    kind: %s
    name: %s`, name, namespace, kubevirtAPIGroup, virtualMachineKind, vmName)
}

func vmCloneManifest(name, sourceName, targetName, namespace string) string {
	return fmt.Sprintf(`apiVersion: clone.kubevirt.io/v1beta1
kind: VirtualMachineClone
metadata:
  name: %s
  namespace: %s
spec:
  source:
    apiGroup: %s
    kind: %s
    name: %s
  target:
    apiGroup: %s
    kind: %s
    name: %s`,
		name, namespace,
		kubevirtAPIGroup, virtualMachineKind, sourceName,
		kubevirtAPIGroup, virtualMachineKind, targetName)
}

func vmPoolManifest(name, namespace string) string {
	selector := name
	return fmt.Sprintf(`apiVersion: pool.kubevirt.io/v1beta1
kind: VirtualMachinePool
metadata:
  name: %s
  namespace: %s
spec:
  replicas: 0
  selector:
    matchLabels:
      select: %s
  virtualMachineTemplate:
    metadata:
      labels:
        select: %s
    spec:
      runStrategy: Manual
      template:
        metadata:
          labels:
            select: %s
        spec:
          domain:
            devices: {}
            resources:
              requests:
                memory: 64Mi
          terminationGracePeriodSeconds: 0`,
		name, namespace, selector, selector, selector)
}

func inventoryVMIManifest(name, namespace string) string {
	return fmt.Sprintf(`apiVersion: kubevirt.io/v1
kind: VirtualMachineInstance
metadata:
  name: %s
  namespace: %s
spec:
  domain:
    devices: {}
    resources:
      requests:
        memory: 64Mi
  terminationGracePeriodSeconds: 0`, name, namespace)
}

func vmimManifest(name, vmiName, namespace string) string {
	return fmt.Sprintf(`apiVersion: kubevirt.io/v1
kind: VirtualMachineInstanceMigration
metadata:
  name: %s
  namespace: %s
spec:
  vmiName: %s`, name, namespace, vmiName)
}
