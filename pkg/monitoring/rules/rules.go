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

package rules

import (
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"

	"github.com/kubevirt/kubevirt-observability-controller/pkg/monitoring/rules/alerts"
	"github.com/kubevirt/kubevirt-observability-controller/pkg/monitoring/rules/recordingrules"
)

var registry = operatorrules.NewRegistry()

func ResetRegistry() {
	registry = operatorrules.NewRegistry()
}

func SetupRules(namespace string, alertsAllowlist, recordingRulesAllowlist map[string]bool) error {
	if err := recordingrules.Register(registry, namespace, recordingRulesAllowlist); err != nil {
		return err
	}

	return alerts.Register(registry, namespace, alertsAllowlist)
}

func HasRegisteredRules() bool {
	return len(registry.ListAlerts()) > 0 || len(registry.ListRecordingRules()) > 0
}

func BuildPrometheusRule(name, namespace string, labels map[string]string) (*promv1.PrometheusRule, error) {
	return registry.BuildPrometheusRule(
		name,
		namespace,
		labels,
	)
}

func ListRecordingRules() []operatorrules.RecordingRule {
	return registry.ListRecordingRules()
}

func ListAlerts() []promv1.Rule {
	return registry.ListAlerts()
}
