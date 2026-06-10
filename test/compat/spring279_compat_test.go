//go:build spring

package compat_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const composeFileSpring279 = "docker-compose.spring279.yml"

func TestSpringBoot279Compat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compat test in short mode")
	}

	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker not available:", err)
	}

	composeDir := compatDir(t)
	composePath := filepath.Join(composeDir, composeFileSpring279)

	t.Logf("compose file: %s", composePath)
	t.Logf("Starting Spring Boot 2.7.9 compat test (may take 3-5 min for first build)...")

	t.Cleanup(func() {
		down := dockerCompose(composeDir, composePath, "down", "--volumes", "--remove-orphans")
		down.Stdout = os.Stdout
		down.Stderr = os.Stderr
		if err := down.Run(); err != nil {
			t.Logf("cleanup down: %v", err)
		}
	})

	up := dockerCompose(composeDir, composePath, "up", "--build", "-d")
	up.Stdout = os.Stdout
	up.Stderr = os.Stderr

	start := time.Now()
	if err := up.Run(); err != nil {
		dumpLogs(t, composeDir, composePath)
		t.Fatalf("docker compose up failed: %v", err)
	}
	t.Logf("containers started in %s", time.Since(start).Round(time.Second))

	t.Log("Waiting for spring279-compat to become healthy...")
	if err := waitHealthy(t, "spring279-compat", 5*time.Minute); err != nil {
		dumpLogs(t, composeDir, composePath)
		t.Fatalf("spring279 did not become healthy: %v", err)
	}
	t.Logf("spring279 healthy after %s", time.Since(start).Round(time.Second))

	var out bytes.Buffer
	probe := exec.Command("docker", "exec", "spring279-compat",
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
		t.Fatalf("COMPAT FAIL: kv_connected nao e true. Response: %s", resp)
	}
	if !strings.Contains(resp, `"cosmos_db_url":"https://cosmos.example.com"`) {
		dumpLogs(t, composeDir, composePath)
		t.Fatalf("COMPAT FAIL: cosmos_db_url nao resolveu. Response: %s", resp)
	}

	elapsed := time.Since(start).Round(time.Second)
	t.Logf("================================================================")
	t.Logf("COMPAT OK — Spring Boot 2.7.9 + Spring Cloud Azure 4.5.0 (%s)", elapsed)
	t.Logf("  - Challenge WWW-Authenticate parseado")
	t.Logf("  - Instance Discovery + OIDC + Token obtido do AAD emulado")
	t.Logf("  - 4/4 secrets carregados (COSMO_DB_URL via app.cosmos.url property resolution)")
	t.Logf("================================================================")
}
