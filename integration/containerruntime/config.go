package containerruntime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	RuntimeAuto           = "auto"
	RuntimeApple          = "apple"
	RuntimeTestcontainers = "testcontainers"

	defaultStartupTimeout = 2 * time.Minute
	defaultPostgresImage  = "postgres:16"
	defaultKeycloakImage  = "quay.io/keycloak/keycloak:24.0.5"
)

type Config struct {
	Runtime        string
	StartupTimeout time.Duration
	PostgresImage  string
	KeycloakImage  string
	AppleBinary    string
	Detected       Detection
}

type Detection struct {
	AppleInstalled       bool
	AppleRunning         bool
	TestcontainersHint   bool
	TestcontainersDetail string
}

func Load(ctx context.Context) (Config, error) {
	detection, appleBinary := detect(ctx)
	return load(os.Getenv, runtime.GOOS, runtime.GOARCH, detection, appleBinary)
}

func (c Config) Summary() string {
	return fmt.Sprintf("selected=%s; detected: %s", c.Runtime, c.Detected.String())
}

func (c Config) Help() string {
	return "set INTEGRATION_CONTAINER_RUNTIME=apple or INTEGRATION_CONTAINER_RUNTIME=testcontainers; " +
		"for Apple Container install `container`, run `container system start`, and retry; " +
		"for Docker/Podman start its Docker-compatible API and configure DOCKER_HOST when needed"
}

func (d Detection) String() string {
	apple := "not installed"
	if d.AppleInstalled {
		apple = "stopped"
	}
	if d.AppleRunning {
		apple = "running"
	}

	testcontainers := "no Docker/Podman endpoint detected"
	if d.TestcontainersHint {
		testcontainers = d.TestcontainersDetail
	}

	return fmt.Sprintf("apple=%s, testcontainers=%s", apple, testcontainers)
}

func load(getenv func(string) string, goos, goarch string, detected Detection, appleBinary string) (Config, error) {
	selected := strings.ToLower(strings.TrimSpace(getenv("INTEGRATION_CONTAINER_RUNTIME")))
	if selected == "" {
		selected = RuntimeAuto
	}
	if selected == "docker" || selected == "podman" {
		selected = RuntimeTestcontainers
	}

	timeout := defaultStartupTimeout
	if value := strings.TrimSpace(getenv("INTEGRATION_CONTAINER_STARTUP_TIMEOUT")); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("INTEGRATION_CONTAINER_STARTUP_TIMEOUT must be a positive Go duration, got %q", value)
		}
		timeout = parsed
	}

	config := Config{
		Runtime:        selected,
		StartupTimeout: timeout,
		PostgresImage:  valueOrDefault(getenv("INTEGRATION_POSTGRES_IMAGE"), defaultPostgresImage),
		KeycloakImage:  valueOrDefault(getenv("INTEGRATION_KEYCLOAK_IMAGE"), defaultKeycloakImage),
		AppleBinary:    appleBinary,
		Detected:       detected,
	}

	switch selected {
	case RuntimeAuto:
		switch {
		case goos == "darwin" && goarch == "arm64" && detected.AppleRunning:
			config.Runtime = RuntimeApple
		case detected.TestcontainersHint:
			config.Runtime = RuntimeTestcontainers
		default:
			return Config{}, selectionError(config, "no usable container runtime was detected")
		}
	case RuntimeApple:
		if goos != "darwin" || goarch != "arm64" {
			return Config{}, selectionError(config, "Apple Container requires macOS on Apple silicon")
		}
		if !detected.AppleInstalled {
			return Config{}, selectionError(config, "the `container` CLI is not installed")
		}
		if !detected.AppleRunning {
			return Config{}, selectionError(config, "Apple Container is not running")
		}
	case RuntimeTestcontainers:
		if !detected.TestcontainersHint {
			return Config{}, selectionError(config, "no Docker/Podman endpoint was detected")
		}
	default:
		return Config{}, selectionError(config, fmt.Sprintf("unsupported runtime %q", selected))
	}

	return config, nil
}

func selectionError(config Config, reason string) error {
	return fmt.Errorf("container runtime selection failed: %s (%s); %s", reason, config.Summary(), config.Help())
}

func valueOrDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func detect(ctx context.Context) (Detection, string) {
	detection := Detection{}
	appleBinary, err := exec.LookPath("container")
	if err == nil {
		detection.AppleInstalled = true
		statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(statusCtx, appleBinary, "system", "status")
		detection.AppleRunning = cmd.Run() == nil
	}

	if endpoint := strings.TrimSpace(os.Getenv("DOCKER_HOST")); endpoint != "" {
		detection.TestcontainersHint = true
		detection.TestcontainersDetail = "DOCKER_HOST is configured"
		return detection, appleBinary
	}

	for _, binary := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(binary); err == nil {
			detection.TestcontainersHint = true
			detection.TestcontainersDetail = binary + " CLI detected"
			return detection, appleBinary
		}
	}

	for _, socket := range dockerSocketCandidates() {
		if info, err := os.Stat(socket); err == nil && info.Mode()&os.ModeSocket != 0 {
			detection.TestcontainersHint = true
			detection.TestcontainersDetail = "Docker-compatible socket detected"
			break
		}
	}
	if !detection.TestcontainersHint && hasTestcontainersDockerHost() {
		detection.TestcontainersHint = true
		detection.TestcontainersDetail = "docker.host is configured in ~/.testcontainers.properties"
	}

	return detection, appleBinary
}

func hasTestcontainersDockerHost() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	contents, err := os.ReadFile(filepath.Join(home, ".testcontainers.properties"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(contents), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "docker.host=") {
			return true
		}
	}
	return false
}

func dockerSocketCandidates() []string {
	candidates := []string{"/var/run/docker.sock"}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".docker", "run", "docker.sock"),
			filepath.Join(home, ".colima", "default", "docker.sock"),
			filepath.Join(home, ".local", "share", "containers", "podman", "machine", "podman.sock"),
		)
	}
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		candidates = append(candidates, filepath.Join(runtimeDir, "podman", "podman.sock"))
	}
	return candidates
}
