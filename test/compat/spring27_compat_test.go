//go:build spring

// Package compat contém testes de compatibilidade com SDKs reais.
// Requer Docker e internet para baixar imagens Maven/Eclipse-Temurin.
//
// Uso:
//   go test -tags=spring ./test/compat/... -v -timeout=15m
//
// Ou via Makefile:
//   make compat/spring27
package compat_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const composeFile = "docker-compose.spring27.yml"

// TestSpringBoot27Compat é o gate definitivo de compatibilidade Spring Boot 2.7.
//
// Sobe:
//   - kvemu (emulador com TLS auto-gen + AAD fake)
//   - kvemu-seed (popula db-password, api-key, connection-string)
//   - spring27 (Spring Boot 2.7 + Spring Cloud Azure 4.5.0 — versão com parser legado)
//
// Passa se e somente se o Spring Boot conseguir:
//  1. Receber o 401 com WWW-Authenticate
//  2. Parsear o challenge sem ArrayIndexOutOfBoundsException
//  3. Carregar 3/3 secrets no contexto Spring (db-password, api-key, connection-string)
//  4. kv_connected=true em /probe
func TestSpringBoot27Compat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compat test in short mode")
	}

	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker not available:", err)
	}

	composeDir := compatDir(t)
	composePath := filepath.Join(composeDir, composeFile)

	t.Logf("compose file: %s", composePath)
	t.Logf("Starting Spring Boot 2.7 compat test (may take 3-5 min for first build)...")

	// cleanup: remove containers e volumes ao final
	t.Cleanup(func() {
		down := dockerCompose(composeDir, composePath, "down", "--volumes", "--remove-orphans")
		down.Stdout = os.Stdout
		down.Stderr = os.Stderr
		if err := down.Run(); err != nil {
			t.Logf("cleanup down: %v", err)
		}
	})

	// sobe em modo detached para evitar que --abort-on-container-exit mate kvemu
	// quando kvemu-seed termina (--exit-code-from implica abort-on-container-exit
	// no Compose v2, o que derrubaria kvemu antes de spring27 conectar)
	up := dockerCompose(composeDir, composePath, "up", "--build", "-d")
	up.Stdout = os.Stdout
	up.Stderr = os.Stderr

	start := time.Now()
	if err := up.Run(); err != nil {
		dumpLogs(t, composeDir, composePath)
		t.Fatalf("docker compose up failed: %v", err)
	}
	t.Logf("containers started in %s", time.Since(start).Round(time.Second))

	// aguarda spring27-compat ficar healthy (healthcheck: curl /probe)
	t.Log("Waiting for spring27-compat to become healthy...")
	if err := waitHealthy(t, "spring27-compat", 5*time.Minute); err != nil {
		dumpLogs(t, composeDir, composePath)
		t.Fatalf("spring27 did not become healthy: %v", err)
	}
	t.Logf("spring27 healthy after %s", time.Since(start).Round(time.Second))

	// coleta output do probe via docker exec
	var out bytes.Buffer
	probe := exec.Command("docker", "exec", "spring27-compat",
		"curl", "-sf", "http://localhost:8080/probe")
	probe.Stdout = io.MultiWriter(os.Stdout, &out)
	probe.Stderr = os.Stderr
	if err := probe.Run(); err != nil {
		dumpLogs(t, composeDir, composePath)
		t.Fatalf("probe curl failed: %v", err)
	}

	resp := out.String()
	t.Logf("probe response: %s", resp)

	if !strings.Contains(resp, `"kv_connected":true`) {
		dumpLogs(t, composeDir, composePath)
		t.Fatalf("COMPAT FAIL: kv_connected nao e true.\n"+
			"Response: %s\n"+
			"Common causes:\n"+
			"  - ArrayIndexOutOfBoundsException no parser do challenge\n"+
			"  - Falha ao obter token do AAD emulado\n"+
			"  - Secrets nao carregados no contexto Spring", resp)
	}
	if !strings.Contains(resp, `"cosmos_db_url":"https://cosmos.example.com"`) {
		dumpLogs(t, composeDir, composePath)
		t.Fatalf("COMPAT FAIL: cosmos_db_url nao resolveu. Response: %s", resp)
	}

	elapsed := time.Since(start).Round(time.Second)
	t.Logf("================================================================")
	t.Logf("COMPAT OK — Spring Boot 2.7 + Spring Cloud Azure 4.5.0 (%s)", elapsed)
	t.Logf("  - WWW-Authenticate challenge parseado sem ArrayIndexOutOfBoundsException")
	t.Logf("  - Token obtido do AAD emulado")
	t.Logf("  - 4/4 secrets carregados (COSMO_DB_URL via app.cosmos.url property resolution)")
	t.Logf("================================================================")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func compatDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func dockerCompose(dir, composePath string, args ...string) *exec.Cmd {
	fullArgs := append([]string{
		"compose",
		"-f", composePath,
		"--project-directory", dir,
	}, args...)
	cmd := exec.Command("docker", fullArgs...)
	cmd.Dir = dir
	return cmd
}

// waitHealthy polls "docker inspect --format={{.State.Health.Status}} <name>"
// até o container ficar healthy ou o timeout expirar.
func waitHealthy(t *testing.T, containerName string, timeout time.Duration) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		out, _ := exec.Command("docker", "inspect",
			"--format={{.State.Health.Status}}",
			containerName,
		).Output()
		status := strings.TrimSpace(string(out))
		t.Logf("[health] %s: %s", containerName, status)
		if status == "healthy" {
			return nil
		}
		if status == "unhealthy" {
			return fmt.Errorf("container %s is unhealthy", containerName)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s to be healthy (last: %s)", containerName, status)
		}
		<-ticker.C
	}
}

func dumpLogs(t *testing.T, composeDir, composePath string) {
	t.Helper()
	logs := dockerCompose(composeDir, composePath, "logs", "--no-color")
	logs.Stdout = os.Stdout
	logs.Stderr = os.Stderr
	_ = logs.Run()
}
