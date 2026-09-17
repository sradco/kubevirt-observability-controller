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
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics Setup", func() {
	BeforeEach(func() {
		Expect(operatormetrics.CleanRegistry()).To(Succeed())
	})

	It("should register all collectors when allowlist is nil", func() {
		err := SetupMetrics(&Stores{}, &Indexers{}, nil)
		Expect(err).ToNot(HaveOccurred())

		metrics := ListMetrics()
		Expect(metrics).ToNot(BeEmpty())
	})

	It("should register no collectors when allowlist is empty (none)", func() {
		err := SetupMetrics(&Stores{}, &Indexers{}, map[string]bool{})
		Expect(err).ToNot(HaveOccurred())

		metrics := ListMetrics()
		Expect(metrics).To(BeEmpty())
	})

	It("should register only allowed metrics", func() {
		allowlist := map[string]bool{
			"kubevirt_vm_info":  true,
			"kubevirt_vmi_info": true,
		}
		err := SetupMetrics(&Stores{}, &Indexers{}, allowlist)
		Expect(err).ToNot(HaveOccurred())

		registered := ListMetrics()
		Expect(registered).To(HaveLen(2))
		for _, m := range registered {
			Expect(allowlist).To(HaveKey(m.GetOpts().Name))
		}
	})

	It("should succeed with unknown metric names in allowlist", func() {
		allowlist := map[string]bool{
			"nonexistent_metric": true,
		}
		err := SetupMetrics(&Stores{}, &Indexers{}, allowlist)
		Expect(err).ToNot(HaveOccurred())

		metrics := ListMetrics()
		Expect(metrics).To(BeEmpty())
	})

	It("should register kubevirt_vmi_migration_info", func() {
		err := SetupMetrics(&Stores{}, &Indexers{}, nil)
		Expect(err).ToNot(HaveOccurred())

		names := map[string]bool{}
		for _, m := range ListMetrics() {
			names[m.GetOpts().Name] = true
		}
		Expect(names).To(HaveKey("kubevirt_vmi_migration_info"))
	})
})
