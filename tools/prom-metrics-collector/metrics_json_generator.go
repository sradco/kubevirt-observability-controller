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

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"

	"github.com/kubevirt/kubevirt-observability-controller/pkg/monitoring/metrics"
	"github.com/kubevirt/kubevirt-observability-controller/pkg/monitoring/rules"
)

// Rare metric names that do not follow operator naming conventions.
// Keep in sync with kubevirt/kubevirt tools/prom-metrics-collector.
var excludedMetrics = map[string]struct{}{
	"kubevirt_vmi_migration_data_total_bytes": {},
}

type RecordingRule struct {
	Record string `json:"record,omitempty"`
	Expr   string `json:"expr,omitempty"`
	Type   string `json:"type,omitempty"`
}

type Output struct {
	MetricFamilies []*dto.MetricFamily `json:"metricFamilies,omitempty"`
	RecordingRules []RecordingRule     `json:"recordingRules,omitempty"`
}

func main() {
	if err := metrics.SetupMetrics(nil, nil, nil); err != nil {
		panic(err)
	}
	if err := rules.SetupRules("", nil, nil); err != nil {
		panic(err)
	}

	metricsList := metrics.ListMetrics()
	rulesList := rules.ListRecordingRules()

	metricFamilies := make([]*dto.MetricFamily, 0, len(metricsList))
	for _, m := range metricsList {
		if _, excluded := excludedMetrics[m.GetOpts().Name]; excluded {
			continue
		}
		opts := m.GetOpts()
		metricType := prometheusType(m.GetBaseType())
		name := opts.Name
		help := opts.Help
		metricFamilies = append(metricFamilies, &dto.MetricFamily{
			Name: &name,
			Help: &help,
			Type: &metricType,
		})
	}

	recNames := make(map[string]struct{}, len(rulesList))
	recRules := make([]RecordingRule, 0, len(rulesList))
	for _, r := range rulesList {
		name := r.GetOpts().Name
		if _, excluded := excludedMetrics[name]; excluded {
			continue
		}
		recNames[name] = struct{}{}
		recRules = append(recRules, RecordingRule{
			Record: name,
			Expr:   r.Expr.String(),
			Type:   strings.ToUpper(string(r.GetType())),
		})
	}

	filteredFamilies := make([]*dto.MetricFamily, 0, len(metricFamilies))
	for _, mf := range metricFamilies {
		if mf.Name == nil {
			continue
		}
		if _, isRec := recNames[*mf.Name]; isRec {
			continue
		}
		filteredFamilies = append(filteredFamilies, mf)
	}

	out := Output{MetricFamilies: filteredFamilies, RecordingRules: recRules}
	jsonBytes, err := json.Marshal(out)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(jsonBytes))
}

func prometheusType(metricType operatormetrics.MetricType) dto.MetricType {
	switch metricType {
	case operatormetrics.CounterType, operatormetrics.CounterVecType:
		return dto.MetricType_COUNTER
	case operatormetrics.GaugeType, operatormetrics.GaugeVecType:
		return dto.MetricType_GAUGE
	case operatormetrics.HistogramType, operatormetrics.HistogramVecType:
		return dto.MetricType_HISTOGRAM
	case operatormetrics.SummaryType, operatormetrics.SummaryVecType:
		return dto.MetricType_SUMMARY
	default:
		return dto.MetricType_UNTYPED
	}
}
