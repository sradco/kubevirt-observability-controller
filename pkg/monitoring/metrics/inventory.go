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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func listStoreObjects[T any](store cache.Store) []*T {
	if store == nil {
		return nil
	}
	cachedObjs := store.List()
	items := make([]*T, 0, len(cachedObjs))
	for _, obj := range cachedObjs {
		typed, ok := obj.(*T)
		if !ok {
			continue
		}
		items = append(items, typed)
	}
	return items
}

func collectUnixTimestamp(
	metric operatormetrics.Metric,
	timestamp metav1.Time,
	labels []string,
) []operatormetrics.CollectorResult {
	if timestamp.IsZero() {
		return nil
	}
	return []operatormetrics.CollectorResult{{
		Metric: metric,
		Value:  float64(timestamp.Unix()),
		Labels: labels,
	}}
}
