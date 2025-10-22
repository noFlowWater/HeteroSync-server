package service

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"time-sync-server/internal/algorithms"
	"time-sync-server/internal/models"
	"time-sync-server/internal/repository"
	"time-sync-server/internal/websocket"
)

type SyncService struct {
	hub  *websocket.Hub
	repo *repository.SQLiteRepository
}

func NewSyncService(hub *websocket.Hub, repo *repository.SQLiteRepository) *SyncService {
	return &SyncService{
		hub:  hub,
		repo: repo,
	}
}

// Device Management
func (s *SyncService) GetConnectedDevices() []*models.Device {
	return s.hub.GetConnectedDevices()
}

// Pairing Management
func (s *SyncService) GetPairings() []*models.Pairing {
	return s.hub.GetPairings()
}

func (s *SyncService) CreatePairing(device1ID, device2ID string) (*models.Pairing, error) {
	if device1ID == device2ID {
		return nil, fmt.Errorf("cannot pair device with itself")
	}
	return s.hub.CreatePairing(device1ID, device2ID)
}

func (s *SyncService) DeletePairing(pairingID string) error {
	return s.hub.DeletePairing(pairingID)
}

// Time Synchronization
func (s *SyncService) RequestTimeSync(pairingID string) (*models.TimeSyncRecord, error) {
	// Request time sync with 5 second timeout
	record, err := s.hub.RequestTimeSync(pairingID, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// Save to database
	if err := s.repo.SaveTimeSyncRecord(record); err != nil {
		return nil, fmt.Errorf("failed to save sync record: %w", err)
	}

	return record, nil
}

// Sync History
func (s *SyncService) GetSyncRecord(id int64) (*models.TimeSyncRecord, error) {
	return s.repo.GetTimeSyncRecord(id)
}

func (s *SyncService) GetSyncRecords(limit, offset int) ([]*models.TimeSyncRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.repo.GetTimeSyncRecords(limit, offset)
}

func (s *SyncService) GetSyncRecordsByDevice(deviceID string, limit, offset int) ([]*models.TimeSyncRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.repo.GetTimeSyncRecordsByDeviceID(deviceID, limit, offset)
}

func (s *SyncService) GetSyncRecordsByTimeRange(startTime, endTime time.Time, limit, offset int) ([]*models.TimeSyncRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.repo.GetTimeSyncRecordsByTimeRange(startTime, endTime, limit, offset)
}

// RequestMultipleTimeSyncs performs NTP-style multi-sampling synchronization
// It takes multiple measurements and applies NTP selection algorithm to find the best offset
func (s *SyncService) RequestMultipleTimeSyncs(req *models.MultiSyncRequest) (*models.AggregatedSyncResult, error) {
	// Apply default values
	if req.SampleCount == 0 || req.SampleCount > 20 {
		req.SampleCount = 8 // NTP standard: 8 samples
	}
	if req.IntervalMs == 0 {
		req.IntervalMs = 200 // 200ms between samples
	}
	if req.TimeoutSec == 0 {
		req.TimeoutSec = 5 // 5 seconds timeout per sample
	}

	timeout := time.Duration(req.TimeoutSec) * time.Second
	interval := time.Duration(req.IntervalMs) * time.Millisecond

	log.Printf("Starting multi-sync for pairing %s: %d samples, %dms interval",
		req.PairingID, req.SampleCount, req.IntervalMs)

	// Perform multiple measurements
	measurements := make([]*models.TimeSyncRecord, 0, req.SampleCount)
	for i := 0; i < req.SampleCount; i++ {
		record, err := s.hub.RequestTimeSync(req.PairingID, timeout)
		if err != nil {
			log.Printf("Sample %d/%d failed: %v", i+1, req.SampleCount, err)
			continue // Skip failed samples
		}

		// Save individual measurement to database
		if err := s.repo.SaveTimeSyncRecord(record); err != nil {
			log.Printf("Failed to save sync record: %v", err)
			// Continue even if DB save fails
		}

		measurements = append(measurements, record)
		log.Printf("Sample %d/%d completed: offset=%dms, rtt1=%dμs, rtt2=%dμs",
			i+1, req.SampleCount,
			getValueOrZero(record.TimeDifference),
			getValueOrZero(record.Device1RTT),
			getValueOrZero(record.Device2RTT))

		// Wait between samples (except for last sample)
		if i < req.SampleCount-1 {
			time.Sleep(interval)
		}
	}

	// Check if we have any valid measurements
	if len(measurements) == 0 {
		return nil, fmt.Errorf("all %d samples failed", req.SampleCount)
	}

	log.Printf("Collected %d/%d valid samples, applying NTP selection algorithm",
		len(measurements), req.SampleCount)

	// Apply NTP selection algorithm
	selector := algorithms.NewNTPSelector(models.NTPFilterConfig{
		MinSamples:       3,
		OutlierThreshold: 2.0, // 2 standard deviations
		TopPercentile:    0.5, // Top 50% by RTT
	})

	result, err := selector.SelectBestMeasurements(measurements)
	if err != nil {
		return nil, fmt.Errorf("NTP selection failed: %w", err)
	}

	// Populate metadata
	result.AggregationID = uuid.New().String()
	result.PairingID = req.PairingID
	result.CreatedAt = time.Now().UnixMilli()

	log.Printf("NTP algorithm completed: best_offset=%dms, confidence=%.2f, valid=%d/%d",
		result.BestOffset, result.Confidence, result.ValidSamples, result.TotalSamples)

	// Save aggregated result to database
	if err := s.repo.SaveAggregatedSyncResult(result); err != nil {
		return nil, fmt.Errorf("failed to save aggregated result: %w", err)
	}

	return result, nil
}

// GetAggregatedSyncResult retrieves a single aggregated sync result by ID
func (s *SyncService) GetAggregatedSyncResult(aggregationID string) (*models.AggregatedSyncResult, error) {
	return s.repo.GetAggregatedSyncResult(aggregationID)
}

// GetAggregatedSyncResults retrieves aggregated sync results for a pairing
func (s *SyncService) GetAggregatedSyncResults(pairingID string, limit, offset int) ([]*models.AggregatedSyncResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.repo.GetAggregatedSyncResultsByPairing(pairingID, limit, offset)
}

// GetAllAggregatedSyncResults retrieves all aggregated sync results
func (s *SyncService) GetAllAggregatedSyncResults(limit, offset int) ([]*models.AggregatedSyncResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.repo.GetAllAggregatedSyncResults(limit, offset)
}

// GetAggregatedSyncResultsByTimeRange retrieves aggregated sync results within a time range
func (s *SyncService) GetAggregatedSyncResultsByTimeRange(startTime, endTime time.Time, limit, offset int) ([]*models.AggregatedSyncResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.repo.GetAggregatedSyncResultsByTimeRange(startTime, endTime, limit, offset)
}

// Helper function to get value or zero for nullable int64 pointers
func getValueOrZero(ptr *int64) int64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
