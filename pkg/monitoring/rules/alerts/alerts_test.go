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

package alerts

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"
)

func TestAlerts(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Alerts Suite")
}

var _ = Describe("Alerts", func() {
	var registry *operatorrules.Registry

	BeforeEach(func() {
		registry = operatorrules.NewRegistry()
	})

	It("should register all alerts without error", func() {
		err := Register(registry, "kubevirt", nil)
		Expect(err).ToNot(HaveOccurred())

		alerts := registry.ListAlerts()
		Expect(alerts).ToNot(BeEmpty())
	})

	It("should include system alerts", func() {
		err := Register(registry, "kubevirt", nil)
		Expect(err).ToNot(HaveOccurred())
		alerts := registry.ListAlerts()

		alertNames := make(map[string]bool)
		for _, a := range alerts {
			alertNames[a.Alert] = true
		}

		Expect(alertNames).To(HaveKey("LowKVMNodesCount"))
		Expect(alertNames).To(HaveKey("KubeVirtNoAvailableNodesToRunVMs"))
	})

	It("should include VM alerts", func() {
		err := Register(registry, "kubevirt", nil)
		Expect(err).ToNot(HaveOccurred())
		alerts := registry.ListAlerts()

		alertNames := make(map[string]bool)
		for _, a := range alerts {
			alertNames[a.Alert] = true
		}

		Expect(alertNames).To(HaveKey("VirtLauncherPodsStuckFailed"))
		Expect(alertNames).To(HaveKey("VMCannotBeEvicted"))
		Expect(alertNames).To(HaveKey("KubeVirtVMIExcessiveMigrations"))
	})

	It("should include component alerts", func() {
		err := Register(registry, "kubevirt", nil)
		Expect(err).ToNot(HaveOccurred())
		alerts := registry.ListAlerts()

		alertNames := make(map[string]bool)
		for _, a := range alerts {
			alertNames[a.Alert] = true
		}

		Expect(alertNames).To(HaveKey("VirtAPIDown"))
		Expect(alertNames).To(HaveKey("VirtControllerDown"))
		Expect(alertNames).To(HaveKey("VirtOperatorDown"))
		Expect(alertNames).To(HaveKey("VirtHandlerDaemonSetRolloutFailing"))
	})

	It("should set the namespace label on component alerts only", func() {
		err := Register(registry, "kubevirt", nil)
		Expect(err).ToNot(HaveOccurred())

		vmAlertNames := map[string]bool{
			"VirtLauncherPodsStuckFailed":                     true,
			"OrphanedVirtualMachineInstances":                 true,
			"VMCannotBeEvicted":                               true,
			"KubeVirtVMIExcessiveMigrations":                  true,
			"OutdatedVirtualMachineInstanceWorkloads":         true,
			"GuestVCPUQueueHighWarning":                       true,
			"GuestVCPUQueueHighCritical":                      true,
			"VirtualMachineStuckInUnhealthyState":             true,
			"VirtualMachineStuckOnNode":                       true,
			"KubeVirtVMGuestMemoryPressure":                   true,
			"GuestFilesystemAlmostOutOfSpace":                 true,
			"VirtualMachineInstanceHasEphemeralHotplugVolume": true,
			"KubeVirtVMGuestMemoryAvailableLow":               true,
		}

		var sawComponent, sawVM bool
		for _, alert := range registry.ListAlerts() {
			if vmAlertNames[alert.Alert] {
				sawVM = true
				Expect(alert.Labels).ToNot(HaveKey("namespace"),
					"VM alert %s must not have a static namespace label",
					alert.Alert)
				continue
			}
			sawComponent = true
			Expect(alert.Labels).To(HaveKeyWithValue("namespace", "kubevirt"),
				"component alert %s should have namespace=kubevirt",
				alert.Alert)
		}
		Expect(sawComponent).To(BeTrue())
		Expect(sawVM).To(BeTrue())
	})

	Context("allowlist filtering", func() {
		It("should register only allowlisted alerts", func() {
			allowlist := map[string]bool{
				"VirtAPIDown":      true,
				"VirtOperatorDown": true,
			}
			err := Register(registry, "kubevirt", allowlist)
			Expect(err).ToNot(HaveOccurred())

			alerts := registry.ListAlerts()
			Expect(alerts).To(HaveLen(2))

			alertNames := make(map[string]bool)
			for _, a := range alerts {
				alertNames[a.Alert] = true
			}

			Expect(alertNames).To(HaveKey("VirtAPIDown"))
			Expect(alertNames).To(HaveKey("VirtOperatorDown"))

			for _, alert := range alerts {
				Expect(alert.Labels).To(HaveKeyWithValue(
					"kubernetes_operator_part_of", "kubevirt"))
				Expect(alert.Labels).To(HaveKeyWithValue(
					"kubernetes_operator_component", "kubevirt"))
				Expect(alert.Labels).To(HaveKeyWithValue("namespace", "kubevirt"))
				Expect(alert.Annotations).To(HaveKey("runbook_url"))
			}
		})

		It("should register no alerts when allowlist is empty map", func() {
			err := Register(registry, "kubevirt", map[string]bool{})
			Expect(err).ToNot(HaveOccurred())

			alerts := registry.ListAlerts()
			Expect(alerts).To(BeEmpty())
		})
	})
})
