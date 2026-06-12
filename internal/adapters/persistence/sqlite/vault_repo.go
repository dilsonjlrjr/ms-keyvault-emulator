package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
)

type VaultRepo struct {
	db *sql.DB
}

func NewVaultRepo(db *sql.DB) *VaultRepo {
	return &VaultRepo{db: db}
}

var _ ports.VaultRepository = (*VaultRepo)(nil)

func (r *VaultRepo) Create(ctx context.Context, v *domain.Vault) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO vault(name, host, dns_name, tenant_id, display_name, created, updated)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.Name, v.Host, fmt.Sprintf("https://%s", v.Host),
		v.TenantID, v.DisplayName, v.Created, v.Updated,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return fmt.Errorf("%w: vault %q", domain.ErrVaultExists, v.Name)
		}
		return err
	}
	return nil
}

func (r *VaultRepo) List(ctx context.Context) ([]*domain.Vault, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT name, host, dns_name, tenant_id, display_name, created, updated
		FROM vault ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vaults []*domain.Vault
	for rows.Next() {
		v := &domain.Vault{}
		var dnsName string
		if err := rows.Scan(&v.Name, &v.Host, &dnsName, &v.TenantID, &v.DisplayName, &v.Created, &v.Updated); err != nil {
			return nil, err
		}
		vaults = append(vaults, v)
	}
	return vaults, rows.Err()
}

func (r *VaultRepo) GetByName(ctx context.Context, name string) (*domain.Vault, error) {
	v := &domain.Vault{}
	var dnsName string
	err := r.db.QueryRowContext(ctx, `
		SELECT name, host, dns_name, tenant_id, display_name, created, updated
		FROM vault WHERE name = ?`, name).
		Scan(&v.Name, &v.Host, &dnsName, &v.TenantID, &v.DisplayName, &v.Created, &v.Updated)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: vault %q", domain.ErrVaultNotFound, name)
	}
	return v, err
}

func (r *VaultRepo) GetByHost(ctx context.Context, host string) (*domain.Vault, error) {
	v := &domain.Vault{}
	var dnsName string
	err := r.db.QueryRowContext(ctx, `
		SELECT name, host, dns_name, tenant_id, display_name, created, updated
		FROM vault WHERE host = ?`, host).
		Scan(&v.Name, &v.Host, &dnsName, &v.TenantID, &v.DisplayName, &v.Created, &v.Updated)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: vault host %q", domain.ErrVaultNotFound, host)
	}
	return v, err
}

func (r *VaultRepo) Delete(ctx context.Context, name string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM vault WHERE name = ?`, name)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w: vault %q", domain.ErrVaultNotFound, name)
	}
	return nil
}

func (r *VaultRepo) Update(ctx context.Context, name string, v *domain.Vault) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE vault SET display_name = ?, updated = ? WHERE name = ?`,
		v.DisplayName, time.Now().Unix(), name)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w: vault %q", domain.ErrVaultNotFound, name)
	}
	return nil
}
