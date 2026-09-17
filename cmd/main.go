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
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	k6tv1 "kubevirt.io/api/core/v1"
	instancetypev1beta1 "kubevirt.io/api/instancetype/v1beta1"
	snapshotv1 "kubevirt.io/api/snapshot/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/kubevirt/kubevirt-observability-controller/pkg/controller"
	"github.com/kubevirt/kubevirt-observability-controller/pkg/monitoring/metrics"
	"github.com/kubevirt/kubevirt-observability-controller/pkg/monitoring/metrics/vmstats"
	"github.com/kubevirt/kubevirt-observability-controller/pkg/tlsutil"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(k6tv1.AddToScheme(scheme))
	utilruntime.Must(instancetypev1beta1.AddToScheme(scheme))
	utilruntime.Must(snapshotv1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var metricsAllowlist string
	var alertsAllowlistRaw string
	var recordingRulesAllowlistRaw string
	var tlsSecurityProfile string
	var tlsMinVersion string
	var tlsCiphers string
	var tlsGroups string
	var tlsOpts []func(*tls.Config)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&tlsSecurityProfile, "tls-security-profile", "",
		"TLS security profile for the metrics server: Old, Intermediate, Modern, or Custom. "+
			"Empty (default) preserves current behavior.")
	flag.StringVar(&tlsMinVersion, "tls-min-version", "",
		"Minimum TLS version (e.g. VersionTLS12, VersionTLS13). "+
			"Only valid with --tls-security-profile=Custom.")
	flag.StringVar(&tlsCiphers, "tls-ciphers", "",
		"Comma-separated list of OpenSSL cipher names. "+
			"Only valid with --tls-security-profile=Custom.")
	flag.StringVar(&tlsGroups, "tls-groups", "",
		"Comma-separated list of OpenSSL group names. "+
			"Only valid with --tls-security-profile=Custom.")
	flag.StringVar(&metricsAllowlist, "metrics-allowlist", "",
		"Comma-separated list of metric names to expose. Empty (default) exposes all. \"none\" disables all custom metrics.")
	flag.StringVar(&alertsAllowlistRaw, "alerts-allowlist", "",
		"Comma-separated list of alert names to include in the PrometheusRule. "+
			"Empty (default) includes all. \"none\" disables all alerts.")
	flag.StringVar(&recordingRulesAllowlistRaw, "recording-rules-allowlist", "",
		"Comma-separated list of recording rule names to include in the PrometheusRule. "+
			"Empty (default) includes all. \"none\" disables all recording rules.")
	var enableVMStats bool
	var vmstatsPort int
	var vmstatsCertPath, vmstatsCertName, vmstatsCertKey string
	flag.BoolVar(&enableVMStats, "enable-vmstats", false,
		"Enable VMStats polling from virt-handler endpoints.")
	flag.IntVar(&vmstatsPort, "vmstats-port", 8187,
		"The port on virt-handler where the /v1/vmstats endpoint is served.")
	flag.StringVar(&vmstatsCertPath, "vmstats-cert-path", "",
		"The directory that contains the client certificate for authenticating to virt-handler vmstats endpoint.")
	flag.StringVar(&vmstatsCertName, "vmstats-cert-name", "tls.crt", "The name of the vmstats client certificate file.")
	flag.StringVar(&vmstatsCertKey, "vmstats-cert-key", "tls.key", "The name of the vmstats client key file.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	if tlsSecurityProfile == "" && (tlsMinVersion != "" || tlsCiphers != "") {
		setupLog.Error(
			fmt.Errorf(
				"--tls-min-version and --tls-ciphers are only valid "+
					"with --tls-security-profile=Custom"),
			"invalid flag combination",
		)
		os.Exit(1)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			os.Exit(1)
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if tlsSecurityProfile != "" {
		tlsConfigFn, err := tlsutil.TLSSecurityProfileToTLSConfig(
			tlsSecurityProfile, tlsMinVersion, tlsCiphers, tlsGroups, logger,
		)
		if err != nil {
			setupLog.Error(err, "invalid TLS security profile configuration")
			os.Exit(1)
		}
		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, tlsConfigFn)
		setupLog.Info("TLS security profile configured",
			"profile", tlsSecurityProfile)
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	allowlist := parseAllowlist(metricsAllowlist)
	alertsAllowlist := parseAllowlist(alertsAllowlistRaw)
	recordingRulesAllowlist := parseAllowlist(recordingRulesAllowlistRaw)
	if err := metrics.SetupMetrics(nil, nil, allowlist); err != nil {
		setupLog.Error(err, "unable to set up metrics")
		os.Exit(1)
	}

	if allowlist != nil {
		registered := metrics.ListMetrics()
		registeredNames := make(map[string]bool, len(registered))
		for _, m := range registered {
			registeredNames[m.GetOpts().Name] = true
		}
		for name := range allowlist {
			if !registeredNames[name] {
				setupLog.Info("metrics-allowlist contains unknown metric name", "metric", name)
			}
		}
		setupLog.Info("metrics allowlist active", "count", len(registered))
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "e0e374f1.kubevirt.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	podNamespace := getPodNamespace()
	serviceAccountName := os.Getenv("POD_SERVICE_ACCOUNT")

	if podNamespace == "" {
		setupLog.Error(fmt.Errorf("POD_NAMESPACE not set"), "missing required environment variable")
		os.Exit(1)
	}

	if serviceAccountName == "" {
		setupLog.Error(fmt.Errorf("POD_SERVICE_ACCOUNT not set"), "missing required environment variable")
		os.Exit(1)
	}

	metricsPort, err := parseMetricsPort(metricsAddr)
	if err != nil {
		setupLog.Error(err, "unable to parse metrics port")
		os.Exit(1)
	}

	if err := (&controller.MetricsResourcesReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Namespace:          podNamespace,
		ServiceAccountName: serviceAccountName,
		MetricsPort:        metricsPort,
		SecureMetrics:      secureMetrics,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MetricsResources")
		os.Exit(1)
	}

	if err := (&controller.PrometheusRuleReconciler{
		Client:                  mgr.GetClient(),
		Scheme:                  mgr.GetScheme(),
		Namespace:               podNamespace,
		AlertsAllowlist:         alertsAllowlist,
		RecordingRulesAllowlist: recordingRulesAllowlist,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PrometheusRule")
		os.Exit(1)
	}

	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		return metrics.SetupInformers(ctx, mgr.GetCache())
	})); err != nil {
		setupLog.Error(err, "unable to add metrics informer setup")
		os.Exit(1)
	}

	if enableVMStats {
		setupVMStats(mgr, vmstatsPort, vmstatsCertPath, vmstatsCertName, vmstatsCertKey, allowlist)
	} else {
		setupLog.Info("VMStats polling disabled")
	}

	// +kubebuilder:scaffold:builder

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}

	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupVMStats(
	mgr manager.Manager,
	vmstatsPort int, vmstatsCertPath, vmstatsCertName, vmstatsCertKey string,
	allowlist map[string]bool,
) {
	if vmstatsCertPath == "" {
		setupLog.Error(fmt.Errorf("vmstats-cert-path is required when vmstats is enabled"), "missing required flag")
		os.Exit(1)
	}

	certFile := filepath.Join(vmstatsCertPath, vmstatsCertName)
	keyFile := filepath.Join(vmstatsCertPath, vmstatsCertKey)
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		setupLog.Error(err, "unable to load vmstats client certificate")
		os.Exit(1)
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
				Certificates:       []tls.Certificate{cert},
			},
		},
	}

	vmStatsClient := vmstats.NewVMStatsClient(httpClient, vmstatsPort)
	statsCache := vmstats.NewStatsCache()

	if err := vmstats.RegisterCollector(statsCache, allowlist); err != nil {
		setupLog.Error(err, "unable to register vmstats collector")
		os.Exit(1)
	}

	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		if !mgr.GetCache().WaitForCacheSync(ctx) {
			return fmt.Errorf("cache sync failed")
		}

		var stores *metrics.Stores
		for {
			stores = metrics.GetStores()
			if stores != nil && stores.VMI != nil && stores.VirtHandlerPod != nil {
				break
			}
			setupLog.Info("waiting for metrics stores to be initialized")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
			}
		}

		poller := vmstats.NewPoller(
			vmstats.PollerConfig{
				PollInterval:  30 * time.Second,
				MaxConcurrent: 10,
				Port:          vmstatsPort,
			},
			statsCache,
			vmStatsClient,
			stores.VMI,
			stores.VirtHandlerPod,
		)
		return poller.Start(ctx)
	})); err != nil {
		setupLog.Error(err, "unable to add vmstats poller")
		os.Exit(1)
	}
}

func getPodNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return strings.TrimSpace(string(data))
	}

	return ""
}

func parseMetricsPort(addr string) (int32, error) {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, fmt.Errorf("parsing metrics-bind-address %q: %w", addr, err)
	}

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parsing metrics port %q: %w", portStr, err)
	}

	return int32(port), nil
}

func parseAllowlist(raw string) map[string]bool {
	if raw == "" {
		return nil
	}
	if raw == "none" {
		return map[string]bool{}
	}

	parts := strings.Split(raw, ",")
	allowlist := make(map[string]bool, len(parts))
	for _, p := range parts {
		if name := strings.TrimSpace(p); name != "" {
			allowlist[name] = true
		}
	}

	return allowlist
}
