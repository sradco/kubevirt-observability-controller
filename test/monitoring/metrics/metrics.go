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
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type MetricLine struct {
	Name   string
	Labels map[string]string
	Value  string
}

func Scrape(namespace, serviceName string) (string, error) {
	localPort, err := freePort()
	if err != nil {
		return "", fmt.Errorf("finding free port: %w", err)
	}

	cmd := exec.Command("kubectl", "port-forward",
		fmt.Sprintf("svc/%s", serviceName),
		fmt.Sprintf("%d:8080", localPort),
		"-n", namespace)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("starting port-forward: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	url := fmt.Sprintf("http://localhost:%d/metrics", localPort)
	client := &http.Client{Timeout: 10 * time.Second}

	var body string
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, httpErr := client.Get(url)
		if httpErr == nil {
			data, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(data)
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if body == "" {
		return "", fmt.Errorf("port-forward to %s/%s never became reachable",
			namespace, serviceName)
	}
	return body, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port, nil
}

func HasMetric(output, name string) bool {
	return len(FindMetric(output, name)) > 0
}

func FindMetric(output, name string) []MetricLine {
	var results []MetricLine
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		ml := parseLine(line)
		if ml.Name == name {
			results = append(results, ml)
		}
	}
	return results
}

func HasLabel(line MetricLine, key, value string) bool {
	return line.Labels[key] == value
}

func FindMetricWithLabels(output, name string, labels map[string]string) []MetricLine {
	var found []MetricLine
	for _, line := range FindMetric(output, name) {
		if lineHasLabels(line, labels) {
			found = append(found, line)
		}
	}
	return found
}

func lineHasLabels(line MetricLine, labels map[string]string) bool {
	for key, value := range labels {
		if line.Labels[key] != value {
			return false
		}
	}
	return true
}

func parseLine(line string) MetricLine {
	ml := MetricLine{Labels: make(map[string]string)}

	braceStart := strings.Index(line, "{")
	braceEnd := strings.LastIndex(line, "}")

	if braceStart == -1 {
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			ml.Name = parts[0]
		}
		if len(parts) >= 2 {
			ml.Value = parts[1]
		}
		return ml
	}

	ml.Name = line[:braceStart]

	labelStr := line[braceStart+1 : braceEnd]
	for _, pair := range splitLabels(labelStr) {
		eqIdx := strings.Index(pair, "=")
		if eqIdx == -1 {
			continue
		}
		key := pair[:eqIdx]
		val := strings.Trim(pair[eqIdx+1:], "\"")
		ml.Labels[key] = val
	}

	remainder := strings.TrimSpace(line[braceEnd+1:])
	if remainder != "" {
		ml.Value = strings.Fields(remainder)[0]
	}

	return ml
}

func splitLabels(s string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	for _, c := range s {
		switch {
		case c == '"':
			inQuotes = !inQuotes
			current.WriteRune(c)
		case c == ',' && !inQuotes:
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}
	return parts
}
