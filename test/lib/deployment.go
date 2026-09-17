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

package lib

import (
	"fmt"
	"strings"
	"time"
)

const (
	ControllerName = "virt-observability-controller"
	MetricsService = "virt-observability-controller-metrics"
)

func rbacManifest(namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: %[1]s
  namespace: %[2]s
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: %[1]s
rules:
- apiGroups: ["kubevirt.io"]
  resources: ["virtualmachines", "virtualmachineinstances", "virtualmachineinstancemigrations", "kubevirts"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["instancetype.kubevirt.io"]
  resources:
  - virtualmachineinstancetypes
  - virtualmachineclusterinstancetypes
  - virtualmachinepreferences
  - virtualmachineclusterpreferences
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["pods", "persistentvolumeclaims"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["snapshot.kubevirt.io"]
  resources: ["virtualmachinesnapshots", "virtualmachinerestores"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["export.kubevirt.io"]
  resources: ["virtualmachineexports"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["clone.kubevirt.io"]
  resources: ["virtualmachineclones"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["pool.kubevirt.io"]
  resources: ["virtualmachinepools"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["controllerrevisions"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["monitoring.coreos.com"]
  resources: ["prometheusrules"]
  verbs: ["get", "list", "watch", "create", "update", "delete"]
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get"]
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get", "list", "watch", "create", "update", "delete"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch", "create", "update", "delete"]
- apiGroups: ["monitoring.coreos.com"]
  resources: ["servicemonitors"]
  verbs: ["get", "list", "watch", "create", "update", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %[1]s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %[1]s
subjects:
- kind: ServiceAccount
  name: %[1]s
  namespace: %[2]s`, ControllerName, namespace)
}

func deploymentManifest(namespace, image string, extraArgs ...string) string {
	args := make([]string, 0, 4+len(extraArgs))
	args = append(args,
		"--metrics-bind-address=:8080",
		"--metrics-secure=false",
		"--health-probe-bind-address=:8081",
		"--leader-elect=false",
	)
	args = append(args, extraArgs...)

	var argsBuilder strings.Builder
	for _, a := range args {
		fmt.Fprintf(&argsBuilder, "\n        - %s", a)
	}
	argsYAML := argsBuilder.String()

	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %[1]s
  template:
    metadata:
      labels:
        app: %[1]s
        app.kubernetes.io/name: %[1]s
        kubevirt.io: %[1]s
    spec:
      serviceAccountName: %[1]s
      containers:
      - name: manager
        image: %[3]s
        imagePullPolicy: Always
        args:%[4]s
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_SERVICE_ACCOUNT
          valueFrom:
            fieldRef:
              fieldPath: spec.serviceAccountName
        ports:
        - containerPort: 8080
          name: metrics
        - containerPort: 8081
          name: health
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
        volumeMounts:
        - name: monitoring-client-cert
          mountPath: /etc/monitoring/clientcertificates
          readOnly: true
      volumes:
      - name: monitoring-client-cert
        secret:
          secretName: kubevirt-virt-handler-monitoring-client-certs
          optional: true`, ControllerName, namespace, image, argsYAML)
}

func DeployController(namespace, image string, extraArgs ...string) error {
	if err := KubectlApply(rbacManifest(namespace)); err != nil {
		return fmt.Errorf("deploying RBAC: %w", err)
	}
	if err := KubectlApply(deploymentManifest(namespace, image, extraArgs...)); err != nil {
		return fmt.Errorf("deploying controller: %w", err)
	}
	return nil
}

func DeleteController(namespace string) error {
	var errs []string
	if err := KubectlDelete(deploymentManifest(namespace, "placeholder")); err != nil {
		errs = append(errs, err.Error())
	}
	if err := KubectlDelete(rbacManifest(namespace)); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("deleting controller: %s", strings.Join(errs, "; "))
	}
	return nil
}

func WaitForControllerReady(namespace string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := Kubectl("get", "pods", "-n", namespace,
			"-l", "app="+ControllerName,
			"-o", "jsonpath={.items[0].status.phase}")
		if err == nil && out == "Running" {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("controller pod did not reach Running phase within %v", timeout)
}

func RedeployController(namespace, image string, extraArgs ...string) error {
	if err := DeleteController(namespace); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	if err := DeployController(namespace, image, extraArgs...); err != nil {
		return err
	}
	return WaitForControllerReady(namespace, 2*time.Minute)
}
