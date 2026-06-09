package app

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// StartPurgeScheduler inicia goroutine que purga automaticamente itens com
// scheduled_purge vencido (soft-deleted há mais de retention_days).
func StartPurgeScheduler(ctx context.Context, db *sql.DB, interval time.Duration) {
	go func() {
		slog.Info("purge scheduler started", "interval", interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runAutoPurge(db)
			case <-ctx.Done():
				slog.Info("purge scheduler stopped")
				return
			}
		}
	}()
}

func runAutoPurge(db *sql.DB) {
	now := time.Now().Unix()
	for _, t := range []string{"secret", "vkey", "vcert"} {
		res, err := db.Exec(
			`DELETE FROM `+t+` WHERE deleted_at IS NOT NULL AND scheduled_purge IS NOT NULL AND scheduled_purge <= ?`,
			now,
		)
		if err != nil {
			slog.Error("auto purge error", "table", t, "err", err)
			continue
		}
		if n, _ := res.RowsAffected(); n > 0 {
			slog.Info("auto purge", "table", t, "rows", n)
		}
	}
}
