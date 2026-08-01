package kubernetes

import (
	"strings"
	"testing"
	"time"
)

func TestConfigFromEnvDefaultsToDisabledInCluster(t *testing.T) {
	cfg, err := ConfigFromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if cfg.Enabled || cfg.AuthMode != AuthModeInCluster || cfg.Source.ClusterDomain != "cluster.local" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.ReconcileInterval != 5*time.Minute || cfg.RequestTimeout != 15*time.Second || cfg.Source.StaleRetention != 7*24*time.Hour {
		t.Fatalf("unexpected duration defaults: %+v", cfg)
	}
}

func TestConfigFromEnvLoadsExplicitKubeconfig(t *testing.T) {
	env := map[string]string{
		"KUBERNETES_DISCOVERY_ENABLED":            "true",
		"KUBERNETES_DISCOVERY_SOURCE_KEY":         "local",
		"KUBERNETES_DISCOVERY_SOURCE_NAME":        "Local cluster",
		"KUBERNETES_DISCOVERY_SITE_ID":            "550e8400-e29b-41d4-a716-446655440000",
		"KUBERNETES_DISCOVERY_AUTH_MODE":          "kubeconfig",
		"KUBERNETES_DISCOVERY_KUBECONFIG_PATH":    "/tmp/kubeconfig",
		"KUBERNETES_DISCOVERY_KUBECONFIG_CONTEXT": "kiac",
		"KUBERNETES_DISCOVERY_NAMESPACES":         "default, apps,default",
		"KUBERNETES_DISCOVERY_CLUSTER_DOMAIN":     "corp.local",
		"KUBERNETES_DISCOVERY_INTERVAL":           "2m",
		"KUBERNETES_DISCOVERY_REQUEST_TIMEOUT":    "7s",
	}
	cfg, err := ConfigFromEnv(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if !cfg.Enabled || cfg.KubeconfigContext != "kiac" || len(cfg.Source.Namespaces) != 2 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestConfigRejectsPartialOrContradictoryScope(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "missing site", env: map[string]string{"KUBERNETES_DISCOVERY_ENABLED": "true", "KUBERNETES_DISCOVERY_SOURCE_KEY": "x", "KUBERNETES_DISCOVERY_NAMESPACES": "default"}, want: "SITE_ID"},
		{name: "mixed all namespaces", env: map[string]string{"KUBERNETES_DISCOVERY_ENABLED": "true", "KUBERNETES_DISCOVERY_SOURCE_KEY": "x", "KUBERNETES_DISCOVERY_SITE_ID": "550e8400-e29b-41d4-a716-446655440000", "KUBERNETES_DISCOVERY_NAMESPACES": "*,default"}, want: "cannot mix"},
		{name: "implicit kubeconfig", env: map[string]string{"KUBERNETES_DISCOVERY_ENABLED": "true", "KUBERNETES_DISCOVERY_SOURCE_KEY": "x", "KUBERNETES_DISCOVERY_SITE_ID": "550e8400-e29b-41d4-a716-446655440000", "KUBERNETES_DISCOVERY_NAMESPACES": "default", "KUBERNETES_DISCOVERY_AUTH_MODE": "kubeconfig"}, want: "KUBECONFIG_PATH"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ConfigFromEnv(func(key string) string { return tt.env[key] })
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}
