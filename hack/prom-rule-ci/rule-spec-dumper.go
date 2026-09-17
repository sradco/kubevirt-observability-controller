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
	"os"

	"github.com/kubevirt/kubevirt-observability-controller/pkg/monitoring/rules"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <output-file>\n", os.Args[0])
		os.Exit(1)
	}

	if err := rules.SetupRules("ci", nil, nil); err != nil {
		panic(err)
	}

	promRule, err := rules.BuildPrometheusRule(
		"kubevirt-observability-rules",
		"ci",
		map[string]string{"app.kubernetes.io/managed-by": "kubevirt-observability-controller"},
	)
	if err != nil {
		panic(err)
	}

	b, err := json.Marshal(promRule.Spec)
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(os.Args[1], b, 0o644); err != nil {
		panic(err)
	}
}
