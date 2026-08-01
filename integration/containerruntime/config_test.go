package containerruntime

import (
	"strings"
	"testing"
	"time"
)

func TestLoadAutoPrefersRunningAppleRuntimeOnAppleSilicon(t *testing.T) {
	config, err := load(env(nil), "darwin", "arm64", Detection{
		AppleInstalled:       true,
		AppleRunning:         true,
		TestcontainersHint:   true,
		TestcontainersDetail: "Docker-compatible endpoint detected",
	}, "/usr/local/bin/container")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.Runtime != RuntimeApple {
		t.Fatalf("expected Apple runtime, got %q", config.Runtime)
	}
	if config.AppleBinary != "/usr/local/bin/container" {
		t.Fatalf("unexpected Apple binary %q", config.AppleBinary)
	}
}

func TestLoadAutoUsesTestcontainersFallback(t *testing.T) {
	config, err := load(env(nil), "linux", "amd64", Detection{
		TestcontainersHint:   true,
		TestcontainersDetail: "Docker-compatible socket detected",
	}, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.Runtime != RuntimeTestcontainers {
		t.Fatalf("expected Testcontainers runtime, got %q", config.Runtime)
	}
}

func TestLoadExplicitTestcontainersUsesDetectedRemoteEndpoint(t *testing.T) {
	config, err := load(env(map[string]string{
		"INTEGRATION_CONTAINER_RUNTIME": "podman",
	}), "linux", "amd64", Detection{
		TestcontainersHint:   true,
		TestcontainersDetail: "DOCKER_HOST is configured",
	}, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.Runtime != RuntimeTestcontainers {
		t.Fatalf("expected Testcontainers runtime, got %q", config.Runtime)
	}
}

func TestLoadExplicitTestcontainersRejectsMissingEndpoint(t *testing.T) {
	_, err := load(env(map[string]string{
		"INTEGRATION_CONTAINER_RUNTIME": "testcontainers",
	}), "linux", "amd64", Detection{}, "")
	if err == nil || !strings.Contains(err.Error(), "no Docker/Podman endpoint was detected") {
		t.Fatalf("expected missing endpoint error, got %v", err)
	}
}

func TestLoadMissingRuntimeHasActionableDiagnostic(t *testing.T) {
	_, err := load(env(nil), "darwin", "arm64", Detection{}, "")
	if err == nil {
		t.Fatal("expected missing runtime error")
	}
	for _, fragment := range []string{"selected=auto", "apple=not installed", "INTEGRATION_CONTAINER_RUNTIME=apple", "container system start", "DOCKER_HOST"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q: %v", fragment, err)
		}
	}
}

func TestLoadRejectsStoppedExplicitAppleRuntime(t *testing.T) {
	_, err := load(env(map[string]string{
		"INTEGRATION_CONTAINER_RUNTIME": "apple",
	}), "darwin", "arm64", Detection{AppleInstalled: true}, "/usr/local/bin/container")
	if err == nil || !strings.Contains(err.Error(), "Apple Container is not running") {
		t.Fatalf("expected stopped runtime error, got %v", err)
	}
}

func TestLoadConfigurationOverrides(t *testing.T) {
	config, err := load(env(map[string]string{
		"INTEGRATION_CONTAINER_RUNTIME":         "testcontainers",
		"INTEGRATION_CONTAINER_STARTUP_TIMEOUT": "45s",
		"INTEGRATION_POSTGRES_IMAGE":            "registry.example/postgres:test",
		"INTEGRATION_KEYCLOAK_IMAGE":            "registry.example/keycloak:test",
	}), "linux", "amd64", Detection{TestcontainersHint: true}, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.StartupTimeout != 45*time.Second {
		t.Fatalf("unexpected timeout %s", config.StartupTimeout)
	}
	if config.PostgresImage != "registry.example/postgres:test" || config.KeycloakImage != "registry.example/keycloak:test" {
		t.Fatalf("unexpected images: %+v", config)
	}
}

func TestLoadRejectsInvalidTimeout(t *testing.T) {
	_, err := load(env(map[string]string{
		"INTEGRATION_CONTAINER_STARTUP_TIMEOUT": "eventually",
	}), "linux", "amd64", Detection{TestcontainersHint: true}, "")
	if err == nil || !strings.Contains(err.Error(), "positive Go duration") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func env(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
