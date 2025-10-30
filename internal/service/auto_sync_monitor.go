package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"time-sync-server/internal/models"
)

// AutoSyncMonitor manages automatic periodic synchronization for pairings
type AutoSyncMonitor struct {
	syncService *SyncService
	jobs        map[string]*autoSyncJobContext
	mu          sync.RWMutex
}

// autoSyncJobContext holds the context and control for a single auto-sync job
type autoSyncJobContext struct {
	job        *models.AutoSyncJob
	cancelFunc context.CancelFunc
	mu         sync.RWMutex
}

// NewAutoSyncMonitor creates a new AutoSyncMonitor instance
func NewAutoSyncMonitor(syncService *SyncService) *AutoSyncMonitor {
	return &AutoSyncMonitor{
		syncService: syncService,
		jobs:        make(map[string]*autoSyncJobContext),
	}
}

// StartAutoSync starts automatic synchronization for a pairing
func (m *AutoSyncMonitor) StartAutoSync(config models.AutoSyncConfig) error {
	// Apply default values
	if config.IntervalSec <= 0 {
		config.IntervalSec = 60 // 60 seconds default
	}
	if config.SampleCount <= 0 {
		config.SampleCount = 8
	}
	if config.IntervalMs <= 0 {
		config.IntervalMs = 200
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running
	if _, exists := m.jobs[config.PairingID]; exists {
		return fmt.Errorf("auto-sync already running for pairing: %s", config.PairingID)
	}

	// Verify pairing exists
	pairings := m.syncService.GetPairings()
	pairingExists := false
	for _, p := range pairings {
		if p.PairingID == config.PairingID {
			pairingExists = true
			break
		}
	}
	if !pairingExists {
		return fmt.Errorf("pairing not found: %s", config.PairingID)
	}

	// Create job context
	ctx, cancel := context.WithCancel(context.Background())
	job := &models.AutoSyncJob{
		PairingID:       config.PairingID,
		Status:          models.AutoSyncStatusRunning,
		Config:          config,
		StartedAt:       time.Now(),
		LastSyncSuccess: true,
		TotalSyncs:      0,
		FailedSyncs:     0,
	}

	jobCtx := &autoSyncJobContext{
		job:        job,
		cancelFunc: cancel,
	}

	m.jobs[config.PairingID] = jobCtx

	// Start background goroutine
	go m.runAutoSync(ctx, jobCtx)

	log.Printf("Auto-sync started for pairing %s (interval: %ds, samples: %d)",
		config.PairingID, config.IntervalSec, config.SampleCount)

	return nil
}

// StopAutoSync stops automatic synchronization for a pairing
func (m *AutoSyncMonitor) StopAutoSync(pairingID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobCtx, exists := m.jobs[pairingID]
	if !exists {
		return fmt.Errorf("auto-sync not running for pairing: %s", pairingID)
	}

	// Cancel the context to stop the goroutine
	jobCtx.cancelFunc()

	// Update status
	jobCtx.mu.Lock()
	jobCtx.job.Status = models.AutoSyncStatusStopped
	jobCtx.mu.Unlock()

	// Remove from active jobs
	delete(m.jobs, pairingID)

	log.Printf("Auto-sync stopped for pairing %s", pairingID)

	return nil
}

// GetStatus returns the status of a specific auto-sync job
func (m *AutoSyncMonitor) GetStatus(pairingID string) (*models.AutoSyncJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobCtx, exists := m.jobs[pairingID]
	if !exists {
		return nil, fmt.Errorf("auto-sync not running for pairing: %s", pairingID)
	}

	jobCtx.mu.RLock()
	defer jobCtx.mu.RUnlock()

	// Return a copy to avoid race conditions
	jobCopy := *jobCtx.job
	return &jobCopy, nil
}

// GetAllStatuses returns the status of all auto-sync jobs
func (m *AutoSyncMonitor) GetAllStatuses() []*models.AutoSyncJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]*models.AutoSyncJob, 0, len(m.jobs))
	for _, jobCtx := range m.jobs {
		jobCtx.mu.RLock()
		jobCopy := *jobCtx.job
		jobCtx.mu.RUnlock()
		statuses = append(statuses, &jobCopy)
	}

	return statuses
}

// Shutdown stops all auto-sync jobs gracefully
func (m *AutoSyncMonitor) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("Shutting down auto-sync monitor (%d jobs)", len(m.jobs))

	for pairingID, jobCtx := range m.jobs {
		jobCtx.cancelFunc()
		jobCtx.mu.Lock()
		jobCtx.job.Status = models.AutoSyncStatusStopped
		jobCtx.mu.Unlock()
		log.Printf("Stopped auto-sync for pairing %s", pairingID)
	}

	// Clear all jobs
	m.jobs = make(map[string]*autoSyncJobContext)
}

// runAutoSync is the background goroutine that performs periodic synchronization
func (m *AutoSyncMonitor) runAutoSync(ctx context.Context, jobCtx *autoSyncJobContext) {
	jobCtx.mu.RLock()
	config := jobCtx.job.Config
	jobCtx.mu.RUnlock()

	log.Printf("Auto-sync goroutine started for pairing %s", config.PairingID)

	// Perform initial synchronization immediately
	log.Printf("Auto-sync performing initial sync for pairing %s", config.PairingID)
	m.performSync(jobCtx)

	// Setup ticker for periodic synchronization
	ticker := time.NewTicker(time.Duration(config.IntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Auto-sync goroutine stopped for pairing %s", config.PairingID)
			return

		case <-ticker.C:
			// Perform periodic synchronization
			m.performSync(jobCtx)
		}
	}
}

// performSync executes a single synchronization attempt
func (m *AutoSyncMonitor) performSync(jobCtx *autoSyncJobContext) {
	jobCtx.mu.RLock()
	config := jobCtx.job.Config
	pairingID := jobCtx.job.PairingID
	jobCtx.mu.RUnlock()

	log.Printf("Auto-sync executing for pairing %s", pairingID)

	// Create multi-sync request
	req := &models.MultiSyncRequest{
		PairingID:   config.PairingID,
		SampleCount: config.SampleCount,
		IntervalMs:  config.IntervalMs,
		TimeoutSec:  5, // Fixed 5 second timeout per sample
	}

	// Execute synchronization
	result, err := m.syncService.RequestMultipleTimeSyncs(req)

	// Update job status
	jobCtx.mu.Lock()
	defer jobCtx.mu.Unlock()

	now := time.Now()
	jobCtx.job.LastSyncAt = &now
	jobCtx.job.TotalSyncs++

	if err != nil {
		jobCtx.job.LastSyncSuccess = false
		jobCtx.job.LastError = err.Error()
		jobCtx.job.FailedSyncs++
		log.Printf("Auto-sync failed for pairing %s: %v", pairingID, err)
	} else {
		jobCtx.job.LastSyncSuccess = true
		jobCtx.job.LastError = ""
		log.Printf("Auto-sync succeeded for pairing %s: offset=%dms, confidence=%.2f",
			pairingID, result.BestOffset, result.Confidence)
	}
}

// IsRunning checks if an auto-sync job is currently running for a pairing
func (m *AutoSyncMonitor) IsRunning(pairingID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobCtx, exists := m.jobs[pairingID]
	if !exists {
		return false
	}

	jobCtx.mu.RLock()
	defer jobCtx.mu.RUnlock()

	return jobCtx.job.Status == models.AutoSyncStatusRunning
}
