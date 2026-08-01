package kubernetes

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
)

func TestLiveServiceList(t *testing.T) {
	kubeconfigPath := os.Getenv("KUBERNETES_LIVE_KUBECONFIG")
	if kubeconfigPath == "" {
		t.Skip("set KUBERNETES_LIVE_KUBECONFIG for an opt-in live cluster check")
	}
	cfg := Config{
		Enabled: true,
		Source: domain.KubernetesSourceConfig{
			Key: "live-test", Name: "Live test", SiteID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
			ClusterDomain: "cluster.local", Namespaces: []string{"*"},
			StaleRetention: 7 * 24 * time.Hour,
		},
		AuthMode: AuthModeKubeconfig, KubeconfigPath: kubeconfigPath,
		KubeconfigContext: os.Getenv("KUBERNETES_LIVE_CONTEXT"),
		ReconcileInterval: time.Minute, RequestTimeout: 15 * time.Second,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	services, err := client.ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	t.Logf("listed %d Services", len(services))
}
