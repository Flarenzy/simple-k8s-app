package kubernetes

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/Flarenzy/simple-k8s-app/internal/domain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type ServiceLister interface {
	ListServices(ctx context.Context) ([]domain.KubernetesServiceSnapshot, error)
}

type Client struct {
	client kubernetes.Interface
	config Config
}

func NewClient(config Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	restConfig, err := buildRESTConfig(config)
	if err != nil {
		return nil, err
	}
	restConfig.Timeout = config.RequestTimeout
	restConfig.UserAgent = "simple-k8s-app-service-discovery"
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	return &Client{client: client, config: config}, nil
}

func NewClientWithInterface(config Config, client kubernetes.Interface) *Client {
	return &Client{client: client, config: config}
}

func buildRESTConfig(config Config) (*rest.Config, error) {
	switch config.AuthMode {
	case AuthModeInCluster:
		result, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("load in-cluster kubernetes credentials: %w", err)
		}
		return result, nil
	case AuthModeKubeconfig:
		rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: config.KubeconfigPath}
		overrides := &clientcmd.ConfigOverrides{CurrentContext: config.KubeconfigContext}
		result, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("load kubernetes kubeconfig: %w", err)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported kubernetes discovery auth mode %q", config.AuthMode)
	}
}

func (c *Client) ListServices(ctx context.Context) ([]domain.KubernetesServiceSnapshot, error) {
	requestCtx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()

	snapshots := make([]domain.KubernetesServiceSnapshot, 0)
	for _, namespace := range c.config.Source.Namespaces {
		if namespace == "*" {
			namespace = metav1.NamespaceAll
		}
		list, err := c.client.CoreV1().Services(namespace).List(requestCtx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list services in namespace %q: %w", displayNamespace(namespace), err)
		}
		for i := range list.Items {
			snapshot, err := serviceToSnapshot(&list.Items[i], c.config.Source.ClusterDomain)
			if err != nil {
				return nil, err
			}
			snapshots = append(snapshots, snapshot)
		}
	}
	sort.Slice(snapshots, func(i, j int) bool {
		left, right := snapshots[i], snapshots[j]
		if left.Namespace != right.Namespace {
			return left.Namespace < right.Namespace
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.UID < right.UID
	})
	return snapshots, nil
}

func serviceToSnapshot(service *corev1.Service, clusterDomain string) (domain.KubernetesServiceSnapshot, error) {
	if service.UID == "" {
		return domain.KubernetesServiceSnapshot{}, fmt.Errorf("service %s/%s has no UID", service.Namespace, service.Name)
	}
	snapshot := domain.KubernetesServiceSnapshot{
		UID:             string(service.UID),
		Namespace:       service.Namespace,
		Name:            service.Name,
		Type:            string(service.Spec.Type),
		ResourceVersion: service.ResourceVersion,
		ExternalName:    service.Spec.ExternalName,
		DNSName:         serviceDNSName(service.Name, service.Namespace, clusterDomain),
		Ports:           make([]domain.KubernetesServicePort, 0, len(service.Spec.Ports)),
		Addresses:       make([]domain.KubernetesServiceAddress, 0),
		Hostnames:       make([]domain.KubernetesServiceHostname, 0),
	}
	for _, port := range service.Spec.Ports {
		targetPort := port.TargetPort.String()
		if targetPort == "0" {
			targetPort = strconv.Itoa(int(port.Port))
		}
		entry := domain.KubernetesServicePort{
			Name: port.Name, Protocol: string(port.Protocol), Port: port.Port,
			TargetPort: targetPort, NodePort: optionalPort(port.NodePort),
		}
		if port.AppProtocol != nil {
			entry.AppProtocol = *port.AppProtocol
		}
		snapshot.Ports = append(snapshot.Ports, entry)
	}

	clusterIPs := service.Spec.ClusterIPs
	if len(clusterIPs) == 0 && service.Spec.ClusterIP != "" {
		clusterIPs = []string{service.Spec.ClusterIP}
	}
	seenAddresses := make(map[string]struct{})
	for _, raw := range clusterIPs {
		if raw == "" || strings.EqualFold(raw, corev1.ClusterIPNone) {
			continue
		}
		address, err := netip.ParseAddr(raw)
		if err != nil {
			return domain.KubernetesServiceSnapshot{}, fmt.Errorf("service %s/%s has invalid cluster IP %q: %w", service.Namespace, service.Name, raw, err)
		}
		appendAddress(&snapshot, seenAddresses, domain.KubernetesServiceAddress{Kind: "cluster_ip", Address: address.Unmap()})
	}
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		if ingress.IP != "" {
			address, err := netip.ParseAddr(ingress.IP)
			if err != nil {
				return domain.KubernetesServiceSnapshot{}, fmt.Errorf("service %s/%s has invalid load balancer IP %q: %w", service.Namespace, service.Name, ingress.IP, err)
			}
			ipMode := ""
			if ingress.IPMode != nil {
				ipMode = string(*ingress.IPMode)
			}
			appendAddress(&snapshot, seenAddresses, domain.KubernetesServiceAddress{Kind: "load_balancer", Address: address.Unmap(), IPMode: ipMode})
		}
		if ingress.Hostname != "" {
			snapshot.Hostnames = append(snapshot.Hostnames, domain.KubernetesServiceHostname{Kind: "load_balancer", Hostname: ingress.Hostname})
		}
	}
	return snapshot, nil
}

func appendAddress(snapshot *domain.KubernetesServiceSnapshot, seen map[string]struct{}, address domain.KubernetesServiceAddress) {
	key := address.Kind + "\x00" + address.Address.String()
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	snapshot.Addresses = append(snapshot.Addresses, address)
}

func optionalPort(port int32) *int32 {
	if port == 0 {
		return nil
	}
	value := port
	return &value
}

func serviceDNSName(name, namespace, clusterDomain string) string {
	relative := name + "." + namespace + ".svc"
	clusterDomain = strings.Trim(strings.TrimSpace(clusterDomain), ".")
	if clusterDomain == "" {
		return relative
	}
	return relative + "." + clusterDomain
}

func displayNamespace(namespace string) string {
	if namespace == metav1.NamespaceAll {
		return "*"
	}
	return namespace
}

var _ ServiceLister = (*Client)(nil)
