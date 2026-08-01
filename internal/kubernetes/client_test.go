package kubernetes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func validTestConfig(namespaces ...string) Config {
	return Config{
		Enabled:           true,
		Source:            domainSource(namespaces...),
		AuthMode:          AuthModeInCluster,
		ReconcileInterval: time.Minute,
		RequestTimeout:    time.Second,
	}
}

func domainSource(namespaces ...string) domain.KubernetesSourceConfig {
	return domain.KubernetesSourceConfig{
		Key: "test", Name: "Test", SiteID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		ClusterDomain: "cluster.test", Namespaces: namespaces,
		StaleRetention: 7 * 24 * time.Hour,
	}
}

func TestClientListsServiceNetworkingMetadata(t *testing.T) {
	appProtocol := "https"
	ipMode := corev1.LoadBalancerIPModeVIP
	clientset := fake.NewClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "commerce", UID: types.UID("uid-1"), ResourceVersion: "17"},
		Spec: corev1.ServiceSpec{
			Type:       corev1.ServiceTypeLoadBalancer,
			ClusterIPs: []string{"10.96.12.4", "fd00::4"},
			Ports:      []corev1.ServicePort{{Name: "https", Protocol: corev1.ProtocolTCP, Port: 443, TargetPort: intstr.FromInt32(8443), AppProtocol: &appProtocol, NodePort: 30443}},
		},
		Status: corev1.ServiceStatus{LoadBalancer: corev1.LoadBalancerStatus{Ingress: []corev1.LoadBalancerIngress{{IP: "192.0.2.10", Hostname: "orders.example.test", IPMode: &ipMode}}}},
	})
	client := NewClientWithInterface(validTestConfig("commerce"), clientset)

	snapshots, err := client.ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot, got %d", len(snapshots))
	}
	snapshot := snapshots[0]
	if snapshot.UID != "uid-1" || snapshot.DNSName != "orders.commerce.svc.cluster.test" || len(snapshot.Addresses) != 3 || len(snapshot.Hostnames) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if len(snapshot.Ports) != 1 || snapshot.Ports[0].TargetPort != "8443" || snapshot.Ports[0].NodePort == nil || *snapshot.Ports[0].NodePort != 30443 {
		t.Fatalf("unexpected ports: %+v", snapshot.Ports)
	}
}

func TestClientRetainsHeadlessAndExternalNameWithoutInventingAddresses(t *testing.T) {
	clientset := fake.NewClientset(
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "headless", Namespace: "apps", UID: types.UID("uid-headless")}, Spec: corev1.ServiceSpec{ClusterIP: corev1.ClusterIPNone}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: "apps", UID: types.UID("uid-external")}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: "upstream.example"}},
	)
	client := NewClientWithInterface(validTestConfig("apps"), clientset)
	snapshots, err := client.ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(snapshots) != 2 || len(snapshots[0].Addresses) != 0 || len(snapshots[1].Addresses) != 0 {
		t.Fatalf("unexpected special-service snapshots: %+v", snapshots)
	}
}

func TestClientDiscardsPartialNamespaceResults(t *testing.T) {
	clientset := fake.NewClientset(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "first", Namespace: "one", UID: types.UID("uid-first")}})
	clientset.PrependReactor("list", "services", func(action ktesting.Action) (bool, runtime.Object, error) {
		if action.GetNamespace() == "two" {
			return true, nil, errors.New("forbidden")
		}
		return false, nil, nil
	})
	client := NewClientWithInterface(validTestConfig("one", "two"), clientset)
	snapshots, err := client.ListServices(context.Background())
	if err == nil || snapshots != nil {
		t.Fatalf("expected partial list failure with no snapshot, got snapshots=%+v err=%v", snapshots, err)
	}
}

func TestClientRejectsMalformedObservedAddress(t *testing.T) {
	clientset := fake.NewClientset(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: "apps", UID: types.UID("uid-bad")}, Spec: corev1.ServiceSpec{ClusterIPs: []string{"not-an-ip"}}})
	client := NewClientWithInterface(validTestConfig("apps"), clientset)
	if _, err := client.ListServices(context.Background()); err == nil {
		t.Fatal("expected malformed address to fail the complete snapshot")
	}
}
