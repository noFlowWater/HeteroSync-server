package service

import (
	"log"

	"time-sync-server/internal/models"
	"time-sync-server/internal/repository"
	"time-sync-server/internal/websocket"
)

// PairingOperator manages automatic pairing restoration when devices reconnect
type PairingOperator struct {
	hub        *websocket.Hub
	repository *repository.SQLiteRepository
	autoSync   *AutoSyncMonitor
}

// NewPairingOperator creates a new PairingOperator instance
func NewPairingOperator(hub *websocket.Hub, repo *repository.SQLiteRepository, autoSync *AutoSyncMonitor) *PairingOperator {
	return &PairingOperator{
		hub:        hub,
		repository: repo,
		autoSync:   autoSync,
	}
}

// OnDeviceConnected is called when a device connects to restore its pairings
func (op *PairingOperator) OnDeviceConnected(deviceID string) {
	// 1. Get all pairings that include this device from DB
	pairings, err := op.repository.GetPairingsByDeviceID(deviceID)
	if err != nil {
		log.Printf("Failed to get pairings for device %s: %v", deviceID, err)
		return
	}

	if len(pairings) == 0 {
		log.Printf("No pairings found for device %s", deviceID)
		return
	}

	log.Printf("Found %d pairing(s) for device %s, checking for restoration", len(pairings), deviceID)

	// 2. For each pairing, check if the other device is also connected
	for _, persistentPairing := range pairings {
		otherDeviceID := op.getOtherDeviceID(persistentPairing, deviceID)

		// 3. Check if the other device is connected
		if !op.hub.IsDeviceConnected(otherDeviceID) {
			log.Printf("Pairing %s cannot be restored: other device %s not connected",
				persistentPairing.PairingID, otherDeviceID)
			continue
		}

		// 4. Check if pairing is already restored (avoid duplicate restoration)
		if op.hub.IsPairingRestored(persistentPairing.PairingID) {
			log.Printf("Pairing %s already restored, skipping", persistentPairing.PairingID)
			continue
		}

		// 5. Restore pairing to in-memory Hub
		pairing := &models.Pairing{
			PairingID: persistentPairing.PairingID,
			Device1ID: persistentPairing.Device1ID,
			Device2ID: persistentPairing.Device2ID,
			CreatedAt: persistentPairing.CreatedAt,
		}

		if err := op.hub.RestorePairing(pairing); err != nil {
			log.Printf("Failed to restore pairing %s: %v", pairing.PairingID, err)
			continue
		}

		log.Printf("✓ Pairing restored: %s (%s <-> %s)",
			pairing.PairingID, pairing.Device1ID, pairing.Device2ID)

		// 6. Restart Auto-Sync with saved configuration
		op.restartAutoSync(persistentPairing)
	}
}

// restartAutoSync restarts Auto-Sync for a restored pairing
func (op *PairingOperator) restartAutoSync(pp *models.PersistentPairing) {
	// Check if Auto-Sync configuration exists
	if pp.AutoSyncIntervalSec == nil || pp.AutoSyncSampleCount == nil || pp.AutoSyncIntervalMs == nil {
		log.Printf("No Auto-Sync configuration found for pairing %s, skipping auto-start", pp.PairingID)
		return
	}

	// Check if Auto-Sync is already running (avoid duplicate start)
	if op.autoSync.IsRunning(pp.PairingID) {
		log.Printf("Auto-Sync already running for pairing %s, skipping", pp.PairingID)
		return
	}

	// Create Auto-Sync config from persisted settings
	config := models.AutoSyncConfig{
		PairingID:   pp.PairingID,
		IntervalSec: *pp.AutoSyncIntervalSec,
		SampleCount: *pp.AutoSyncSampleCount,
		IntervalMs:  *pp.AutoSyncIntervalMs,
	}

	// Start Auto-Sync
	if err := op.autoSync.StartAutoSync(config); err != nil {
		log.Printf("Failed to restart Auto-Sync for pairing %s: %v", pp.PairingID, err)
		return
	}

	log.Printf("✓ Auto-Sync automatically restarted for pairing %s (interval: %ds, samples: %d)",
		pp.PairingID, config.IntervalSec, config.SampleCount)
}

// getOtherDeviceID returns the other device ID in the pairing
func (op *PairingOperator) getOtherDeviceID(pairing *models.PersistentPairing, myDeviceID string) string {
	if pairing.Device1ID == myDeviceID {
		return pairing.Device2ID
	}
	return pairing.Device1ID
}
