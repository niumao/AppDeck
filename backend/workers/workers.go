package workers

import (
	"context"
	"time"

	"appdeck/config"
	"appdeck/logger"
	"appdeck/services"
)

func StartAll(ctx context.Context, cfg *config.Config) {
	moonlightHostService := services.GetMoonlightHostService()
	rechargeService := services.NewRechargeService(cfg)

	backupService := services.InitBackupService(&cfg.Database, "./data/backups")
	backupService.StartScheduler()

	// Billing no longer runs on a shared global ticker (services.SessionService's
	// scheduleSessionExpiry/runSessionTick pair drives each session with its own
	// self-rescheduling timer, started on bind/login and stopped on session end).
	go runStaleHostCleanup(ctx, moonlightHostService)
	go runExpiredOrderCleanup(ctx, rechargeService)
	go func() {
		<-ctx.Done()
		backupService.StopScheduler()
		logger.Info("workers", "Backup scheduler stopped")
	}()
}

func runStaleHostCleanup(ctx context.Context, svc *services.MoonlightHostService) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("workers", "Stale host cleanup stopped")
			return
		case <-ticker.C:
			if err := svc.ReleaseStaleHosts(12 * time.Hour); err != nil {
				logger.Error("workers", "Error releasing stale hosts: %v", err)
			}
		}
	}
}

func runExpiredOrderCleanup(ctx context.Context, svc *services.RechargeService) {
	if err := svc.CancelExpiredOrders(); err != nil {
		logger.Error("workers", "Error cancelling expired orders on startup: %v", err)
	}
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("workers", "Expired order cleanup stopped")
			return
		case <-ticker.C:
			if err := svc.CancelExpiredOrders(); err != nil {
				logger.Error("workers", "Error cancelling expired orders: %v", err)
			}
		}
	}
}
