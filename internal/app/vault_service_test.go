package app

import (
	"context"
	"errors"
	"testing"

	"github.com/dilsonrabelo/kvemu/internal/domain"
)

func newTestVaultService() *VaultService {
	return NewVaultService(newMockVaultRepo(), "kvemu.local", "13000")
}

func TestVaultService_Create_Success(t *testing.T) {
	svc := newTestVaultService()
	v, err := svc.Create(context.Background(), "test-vault", "Test Vault", "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Name != "test-vault" {
		t.Errorf("name = %q, want %q", v.Name, "test-vault")
	}
	if v.Host != "test-vault.kvemu.local:13000" {
		t.Errorf("host = %q, want %q", v.Host, "test-vault.kvemu.local:13000")
	}
	if v.DisplayName != "Test Vault" {
		t.Errorf("display_name = %q, want %q", v.DisplayName, "Test Vault")
	}
	if v.TenantID != "tenant-1" {
		t.Errorf("tenant_id = %q, want %q", v.TenantID, "tenant-1")
	}
	if v.Created == 0 || v.Updated == 0 {
		t.Error("timestamps not set")
	}
}

func TestVaultService_Create_EmptyDisplayName(t *testing.T) {
	svc := newTestVaultService()
	v, err := svc.Create(context.Background(), "prod", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.DisplayName != "prod" {
		t.Errorf("display_name = %q, want %q", v.DisplayName, "prod")
	}
}

func TestVaultService_Create_InvalidName(t *testing.T) {
	svc := newTestVaultService()
	invalid := []string{
		"", "-test", "test-", "ABCD", "Test-Vault",
		"a-very-long-name-that-exceeds-twenty-four-chars",
	}
	for _, name := range invalid {
		_, err := svc.Create(context.Background(), name, "", "")
		if err == nil {
			t.Errorf("expected error for name %q, got nil", name)
		}
	}
}

func TestVaultService_Create_Duplicate(t *testing.T) {
	svc := newTestVaultService()
	_, err := svc.Create(context.Background(), "my-vault", "", "")
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	_, err = svc.Create(context.Background(), "my-vault", "", "")
	if err == nil {
		t.Fatal("expected error on duplicate")
	}
	if !errors.Is(err, domain.ErrVaultExists) {
		t.Errorf("expected ErrVaultExists, got %v", err)
	}
}

func TestVaultService_List_Empty(t *testing.T) {
	svc := newTestVaultService()
	vaults, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vaults) != 0 {
		t.Errorf("expected 0 vaults, got %d", len(vaults))
	}
}

func TestVaultService_List_WithVaults(t *testing.T) {
	svc := newTestVaultService()
	svc.Create(context.Background(), "prod", "", "")
	svc.Create(context.Background(), "staging", "", "")

	vaults, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vaults) != 2 {
		t.Fatalf("expected 2 vaults, got %d", len(vaults))
	}
	if vaults[0].Name != "prod" {
		t.Errorf("expected first vault 'prod', got %q", vaults[0].Name)
	}
	if vaults[1].Name != "staging" {
		t.Errorf("expected second vault 'staging', got %q", vaults[1].Name)
	}
}

func TestVaultService_GetByName_NotFound(t *testing.T) {
	svc := newTestVaultService()
	_, err := svc.GetByName(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrVaultNotFound) {
		t.Errorf("expected ErrVaultNotFound, got %v", err)
	}
}

func TestVaultService_GetByName_Found(t *testing.T) {
	svc := newTestVaultService()
	created, _ := svc.Create(context.Background(), "prod", "Production", "t1")

	v, err := svc.GetByName(context.Background(), "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Name != created.Name {
		t.Errorf("name = %q, want %q", v.Name, created.Name)
	}
}

func TestVaultService_GetByHost_Found(t *testing.T) {
	svc := newTestVaultService()
	svc.Create(context.Background(), "staging", "", "")

	v, err := svc.GetByHost(context.Background(), "staging.kvemu.local:13000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Name != "staging" {
		t.Errorf("name = %q, want %q", v.Name, "staging")
	}
}

func TestVaultService_GetByHost_NotFound(t *testing.T) {
	svc := newTestVaultService()
	_, err := svc.GetByHost(context.Background(), "ghost.kvemu.local:13000")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVaultService_Delete_Success(t *testing.T) {
	svc := newTestVaultService()
	svc.Create(context.Background(), "temp", "", "")

	err := svc.Delete(context.Background(), "temp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.GetByName(context.Background(), "temp")
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestVaultService_Delete_DefaultVault(t *testing.T) {
	svc := newTestVaultService()
	svc.Create(context.Background(), "vault", "", "")

	err := svc.Delete(context.Background(), "vault")
	if err == nil {
		t.Fatal("expected error deleting default vault")
	}
}

func TestDomain_VaultNameFromHost(t *testing.T) {
	tests := []struct {
		host, baseDomain, want string
	}{
		{"prod.kvemu.local:13000", "kvemu.local", "prod"},
		{"staging.kvemu.local:13000", "kvemu.local", "staging"},
		{"vault.kvemu.local", "kvemu.local", "vault"},
		{"my-vault.kvemu.local:13000", "kvemu.local", "my-vault"},
		{"localhost:13000", "kvemu.local", ""},
		{"kvemu.local:13000", "kvemu.local", ""},
		{"127.0.0.1:13000", "kvemu.local", ""},
		{"login.microsoftonline.com", "kvemu.local", ""},
		{"prod.azure.net:443", "azure.net", "prod"},
	}
	for _, tt := range tests {
		got := domain.VaultNameFromHost(tt.host, tt.baseDomain)
		if got != tt.want {
			t.Errorf("VaultNameFromHost(%q, %q) = %q, want %q", tt.host, tt.baseDomain, got, tt.want)
		}
	}
}

func TestDomain_BuildVaultHost(t *testing.T) {
	got := domain.BuildVaultHost("prod", "kvemu.local", "13000")
	want := "prod.kvemu.local:13000"
	if got != want {
		t.Errorf("BuildVaultHost = %q, want %q", got, want)
	}
}
