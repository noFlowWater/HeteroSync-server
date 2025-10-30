package models

import "time"

// DeviceType represents the type of device
type DeviceType string

const (
	DeviceTypePSG    DeviceType = "PSG"
	DeviceTypeWatch  DeviceType = "WATCH"
	DeviceTypeMobile DeviceType = "MOBILE"
)

// SyncStatus represents the status of a time synchronization
type SyncStatus string

const (
	SyncStatusSuccess SyncStatus = "SUCCESS"
	SyncStatusPartial SyncStatus = "PARTIAL"
	SyncStatusFailed  SyncStatus = "FAILED"
)

// Device represents a connected device
type Device struct {
	DeviceID    string     `json:"deviceId"`
	DeviceType  DeviceType `json:"deviceType"`
	ConnectedAt time.Time  `json:"connectedAt"`
}

// DeviceHealth represents the health status of a connected device
type DeviceHealth struct {
	DeviceID       string     `json:"deviceId"`
	DeviceType     DeviceType `json:"deviceType"`
	ConnectedAt    time.Time  `json:"connectedAt"`
	LastPingSent   time.Time  `json:"lastPingSent"`
	LastPongRecv   time.Time  `json:"lastPongRecv"`
	LastRTT        int64      `json:"lastRtt"`        // milliseconds
	IsHealthy      bool       `json:"isHealthy"`      // true if PONG received within threshold
	TimeSinceLastPong int64   `json:"timeSinceLastPong"` // milliseconds
}

// Pairing represents a pairing between two devices (in-memory)
type Pairing struct {
	PairingID string    `json:"pairingId"`
	Device1ID string    `json:"device1Id"`
	Device2ID string    `json:"device2Id"`
	CreatedAt time.Time `json:"createdAt"`
}

// PersistentPairing represents a pairing stored in the database (includes Auto-Sync config)
type PersistentPairing struct {
	PairingID string    `json:"pairingId"`
	Device1ID string    `json:"device1Id"`
	Device2ID string    `json:"device2Id"`
	CreatedAt time.Time `json:"createdAt"`

	// Auto-Sync configuration (nullable)
	AutoSyncIntervalSec *int `json:"autoSyncIntervalSec,omitempty"`
	AutoSyncSampleCount *int `json:"autoSyncSampleCount,omitempty"`
	AutoSyncIntervalMs  *int `json:"autoSyncIntervalMs,omitempty"`
}

// TimeSyncRecord represents a time synchronization record
type TimeSyncRecord struct {
	ID                 int64      `json:"id"`
	Device1ID          string     `json:"device1Id"`
	Device1Type        DeviceType `json:"device1Type"`
	Device1Timestamp   *int64     `json:"device1Timestamp"` // Nullable for timeout, Milliseconds
	Device2ID          string     `json:"device2Id"`
	Device2Type        DeviceType `json:"device2Type"`
	Device2Timestamp   *int64     `json:"device2Timestamp"` // Nullable for timeout, Milliseconds
	ServerRequestTime  int64      `json:"serverRequestTime"` // Milliseconds
	ServerResponseTime *int64     `json:"serverResponseTime"` // Nullable, Milliseconds
	// RTT (Round-Trip Time) measurements in microseconds
	Device1RTT         *int64     `json:"device1Rtt,omitempty"` // Device1 RTT (μs)
	Device2RTT         *int64     `json:"device2Rtt,omitempty"` // Device2 RTT (μs)
	// Time difference (RAW, no network compensation)
	// Network delay compensation is applied by NTPSelector during multi-sampling
	TimeDifference     *int64     `json:"timeDifference,omitempty"` // Raw time diff: Device1Time - Device2Time (ms)
	Status             SyncStatus `json:"status"`
	ErrorMessage       *string    `json:"errorMessage,omitempty"`
	CreatedAt          int64      `json:"createdAt"` // Milliseconds
}

// WebSocket Message Types
type MessageType string

const (
	MessageTypeConnected    MessageType = "CONNECTED"
	MessageTypeTimeRequest  MessageType = "TIME_REQUEST"
	MessageTypeTimeResponse MessageType = "TIME_RESPONSE"
	MessageTypeError        MessageType = "ERROR"
	MessageTypePing         MessageType = "PING"
	MessageTypePong         MessageType = "PONG"
)

// WebSocket Messages
type WSMessage struct {
	Type MessageType `json:"type"`
}

type ConnectedMessage struct {
	Type       MessageType `json:"type"`
	DeviceID   string      `json:"deviceId"`
	ServerTime int64       `json:"serverTime"`
}

type TimeRequestMessage struct {
	Type      MessageType `json:"type"`
	RequestID string      `json:"requestId"`
	PairingID string      `json:"pairingId"`
}

type TimeResponseMessage struct {
	Type      MessageType `json:"type"`
	RequestID string      `json:"requestId"`
	Timestamp int64       `json:"timestamp"`
}

type ErrorMessage struct {
	Type    MessageType `json:"type"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
}

type PingMessage struct {
	Type      MessageType `json:"type"`
	Timestamp int64       `json:"timestamp"`
}

type PongMessage struct {
	Type      MessageType `json:"type"`
	Timestamp int64       `json:"timestamp"`
}

// NTP Multi-Sampling Models

// AggregatedSyncResult represents the result of NTP-style multi-sampling synchronization
type AggregatedSyncResult struct {
	AggregationID string `json:"aggregation_id"`
	PairingID     string `json:"pairing_id"`

	// Final calculated results
	BestOffset   int64   `json:"best_offset"`   // Best offset in milliseconds
	MedianOffset int64   `json:"median_offset"` // Median offset in milliseconds
	MeanOffset   float64 `json:"mean_offset"`   // Mean offset in milliseconds

	// Statistical information
	OffsetStdDev float64 `json:"offset_std_dev"` // Standard deviation of offsets
	MinRTT       int64   `json:"min_rtt"`        // Minimum RTT in microseconds
	MaxRTT       int64   `json:"max_rtt"`        // Maximum RTT in microseconds
	MeanRTT      float64 `json:"mean_rtt"`       // Mean RTT in microseconds

	// Quality metrics
	Confidence float64 `json:"confidence"` // Confidence score 0.0 ~ 1.0
	Jitter     float64 `json:"jitter"`     // RTT variability in microseconds

	// Measurement information
	TotalSamples int `json:"total_samples"`  // Total number of samples attempted
	ValidSamples int `json:"valid_samples"`  // Number of valid samples used
	OutlierCount int `json:"outlier_count"`  // Number of outliers removed

	// All measurement records
	Measurements []*TimeSyncRecord `json:"measurements"`

	// Metadata
	CreatedAt int64 `json:"created_at"` // Milliseconds
}

// MultiSyncRequest represents a request for NTP-style multi-sampling
type MultiSyncRequest struct {
	PairingID   string `json:"pairing_id" binding:"required"`
	SampleCount int    `json:"sample_count"` // Default: 8
	IntervalMs  int    `json:"interval_ms"`  // Interval between samples in ms, default: 200
	TimeoutSec  int    `json:"timeout_sec"`  // Timeout for each sample in seconds, default: 5
}

// NTPFilterConfig represents configuration for NTP filtering algorithm
type NTPFilterConfig struct {
	MinSamples       int     `json:"min_samples"`        // Minimum valid samples required
	OutlierThreshold float64 `json:"outlier_threshold"`  // Outlier detection threshold (stddev multiplier)
	TopPercentile    float64 `json:"top_percentile"`     // Top N% of samples by RTT to select (0.5 = 50%)
}

// SampleAnalysis represents analysis of a single sync sample for NTP algorithm
type SampleAnalysis struct {
	Record         *TimeSyncRecord `json:"record"`
	TotalRTT       int64           `json:"total_rtt"`       // Device1RTT + Device2RTT
	RTTDifference  int64           `json:"rtt_difference"`  // |Device1RTT - Device2RTT|
	Offset         int64           `json:"offset"`          // TimeDifference
	IsOutlier      bool            `json:"is_outlier"`      // Whether this sample is an outlier
	SelectionScore float64         `json:"selection_score"` // Score for selection (lower is better)
}

// API Request/Response Models
type CreatePairingRequest struct {
	Device1ID string `json:"device1Id" binding:"required"`
	Device2ID string `json:"device2Id" binding:"required"`

	// Optional auto-sync configuration (uses server defaults if not provided)
	AutoSyncIntervalSec *int `json:"autoSyncIntervalSec,omitempty"` // Optional: interval between syncs in seconds
	AutoSyncSampleCount *int `json:"autoSyncSampleCount,omitempty"` // Optional: number of samples per sync
	AutoSyncIntervalMs  *int `json:"autoSyncIntervalMs,omitempty"`  // Optional: interval between samples in ms
}

type CreatePairingResponse struct {
	PairingID string `json:"pairingId"`
}

type SyncResponse struct {
	Success bool            `json:"success"`
	Record  *TimeSyncRecord `json:"record,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type MultiSyncResponse struct {
	Success bool                  `json:"success"`
	Result  *AggregatedSyncResult `json:"result,omitempty"`
	Error   string                `json:"error,omitempty"`
}

// Auto-Sync Monitor Models

// AutoSyncStatus represents the status of an auto-sync job
type AutoSyncStatus string

const (
	AutoSyncStatusRunning AutoSyncStatus = "RUNNING"
	AutoSyncStatusStopped AutoSyncStatus = "STOPPED"
	AutoSyncStatusFailed  AutoSyncStatus = "FAILED"
)

// AutoSyncConfig represents configuration for auto-sync monitoring
type AutoSyncConfig struct {
	PairingID   string `json:"pairing_id" binding:"required"`
	IntervalSec int    `json:"interval_sec"` // Interval between syncs in seconds, default: 60
	SampleCount int    `json:"sample_count"` // Number of samples per sync, default: 8
	IntervalMs  int    `json:"interval_ms"`  // Interval between samples in ms, default: 200
}

// AutoSyncJob represents a running auto-sync job
type AutoSyncJob struct {
	PairingID       string         `json:"pairing_id"`
	Status          AutoSyncStatus `json:"status"`
	Config          AutoSyncConfig `json:"config"`
	StartedAt       time.Time      `json:"started_at"`
	LastSyncAt      *time.Time     `json:"last_sync_at,omitempty"`
	LastSyncSuccess bool           `json:"last_sync_success"`
	LastError       string         `json:"last_error,omitempty"`
	TotalSyncs      int            `json:"total_syncs"`
	FailedSyncs     int            `json:"failed_syncs"`
}

// AutoSyncStartRequest represents a request to start auto-sync
type AutoSyncStartRequest struct {
	PairingID   string `json:"pairing_id" binding:"required"`
	IntervalSec int    `json:"interval_sec"` // Default: 60
	SampleCount int    `json:"sample_count"` // Default: 8
	IntervalMs  int    `json:"interval_ms"`  // Default: 200
}

// AutoSyncStatusResponse represents the response for auto-sync status
type AutoSyncStatusResponse struct {
	Jobs []*AutoSyncJob `json:"jobs"`
}
