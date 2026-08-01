package kubernetes

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	AuthModeInCluster  = "in_cluster"
	AuthModeKubeconfig = "kubeconfig"
)

type Config struct {
	Enabled           bool
	Source            domain.KubernetesSourceConfig
	AuthMode          string
	KubeconfigPath    string
	KubeconfigContext string
	ReconcileInterval time.Duration
	RequestTimeout    time.Duration
}

func ConfigFromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{
		AuthMode:          valueOrDefault(getenv("KUBERNETES_DISCOVERY_AUTH_MODE"), AuthModeInCluster),
		KubeconfigPath:    strings.TrimSpace(getenv("KUBERNETES_DISCOVERY_KUBECONFIG_PATH")),
		KubeconfigContext: strings.TrimSpace(getenv("KUBERNETES_DISCOVERY_KUBECONFIG_CONTEXT")),
		ReconcileInterval: 5 * time.Minute,
		RequestTimeout:    15 * time.Second,
		Source: domain.KubernetesSourceConfig{
			Key:            strings.TrimSpace(getenv("KUBERNETES_DISCOVERY_SOURCE_KEY")),
			Name:           strings.TrimSpace(getenv("KUBERNETES_DISCOVERY_SOURCE_NAME")),
			ClusterDomain:  valueOrDefault(getenv("KUBERNETES_DISCOVERY_CLUSTER_DOMAIN"), "cluster.local"),
			Namespaces:     parseList(getenv("KUBERNETES_DISCOVERY_NAMESPACES")),
			StaleRetention: 7 * 24 * time.Hour,
		},
	}

	if raw := strings.TrimSpace(getenv("KUBERNETES_DISCOVERY_ENABLED")); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("KUBERNETES_DISCOVERY_ENABLED: %w", err)
		}
		cfg.Enabled = enabled
	}
	if raw := strings.TrimSpace(getenv("KUBERNETES_DISCOVERY_SITE_ID")); raw != "" {
		siteID, err := uuid.Parse(raw)
		if err != nil {
			return Config{}, fmt.Errorf("KUBERNETES_DISCOVERY_SITE_ID: %w", err)
		}
		cfg.Source.SiteID = siteID
	}
	if cfg.Source.Name == "" {
		cfg.Source.Name = cfg.Source.Key
	}
	var err error
	if cfg.ReconcileInterval, err = parseDuration(getenv("KUBERNETES_DISCOVERY_INTERVAL"), cfg.ReconcileInterval); err != nil {
		return Config{}, fmt.Errorf("KUBERNETES_DISCOVERY_INTERVAL: %w", err)
	}
	if cfg.RequestTimeout, err = parseDuration(getenv("KUBERNETES_DISCOVERY_REQUEST_TIMEOUT"), cfg.RequestTimeout); err != nil {
		return Config{}, fmt.Errorf("KUBERNETES_DISCOVERY_REQUEST_TIMEOUT: %w", err)
	}
	if cfg.Source.StaleRetention, err = parseDuration(getenv("KUBERNETES_DISCOVERY_STALE_RETENTION"), cfg.Source.StaleRetention); err != nil {
		return Config{}, fmt.Errorf("KUBERNETES_DISCOVERY_STALE_RETENTION: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Source.Key == "" {
		return fmt.Errorf("KUBERNETES_DISCOVERY_SOURCE_KEY is required when discovery is enabled")
	}
	if c.Source.SiteID == uuid.Nil {
		return fmt.Errorf("KUBERNETES_DISCOVERY_SITE_ID is required when discovery is enabled")
	}
	clusterDomain := strings.Trim(strings.TrimSpace(c.Source.ClusterDomain), ".")
	if problems := utilvalidation.IsDNS1123Subdomain(clusterDomain); len(problems) > 0 {
		return fmt.Errorf("invalid kubernetes cluster domain %q: %s", c.Source.ClusterDomain, strings.Join(problems, ", "))
	}
	if len(c.Source.Namespaces) == 0 {
		return fmt.Errorf("KUBERNETES_DISCOVERY_NAMESPACES is required when discovery is enabled")
	}
	if len(c.Source.Namespaces) > 1 {
		for _, namespace := range c.Source.Namespaces {
			if namespace == "*" {
				return fmt.Errorf("KUBERNETES_DISCOVERY_NAMESPACES cannot mix * with named namespaces")
			}
		}
	}
	for _, namespace := range c.Source.Namespaces {
		if namespace == "*" {
			continue
		}
		if problems := utilvalidation.IsDNS1123Label(namespace); len(problems) > 0 {
			return fmt.Errorf("invalid kubernetes namespace %q: %s", namespace, strings.Join(problems, ", "))
		}
	}
	switch c.AuthMode {
	case AuthModeInCluster:
		if c.KubeconfigPath != "" || c.KubeconfigContext != "" {
			return fmt.Errorf("kubeconfig path/context cannot be set with in_cluster auth")
		}
	case AuthModeKubeconfig:
		if c.KubeconfigPath == "" {
			return fmt.Errorf("KUBERNETES_DISCOVERY_KUBECONFIG_PATH is required with kubeconfig auth")
		}
	default:
		return fmt.Errorf("unsupported kubernetes discovery auth mode %q", c.AuthMode)
	}
	if c.ReconcileInterval <= 0 {
		return fmt.Errorf("kubernetes discovery interval must be positive")
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("kubernetes discovery request timeout must be positive")
	}
	if c.Source.StaleRetention <= 0 {
		return fmt.Errorf("kubernetes discovery stale retention must be positive")
	}
	return nil
}

func parseList(value string) []string {
	seen := make(map[string]struct{})
	items := make([]string, 0)
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		items = append(items, item)
	}
	return items
}

func parseDuration(value string, fallback time.Duration) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}

func valueOrDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}
