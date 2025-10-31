package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"time-sync-server/config"
	"time-sync-server/internal/models"
	"time-sync-server/internal/service"
	ws "time-sync-server/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development
		// In production, you should restrict this
		return true
	},
}

type Handler struct {
	syncService     *service.SyncService
	autoSyncMonitor *service.AutoSyncMonitor
	hub             *ws.Hub
	config          *config.Config
	repository      service.Repository
}

func NewHandler(syncService *service.SyncService, autoSyncMonitor *service.AutoSyncMonitor, hub *ws.Hub, cfg *config.Config, repo service.Repository) *Handler {
	return &Handler{
		syncService:     syncService,
		autoSyncMonitor: autoSyncMonitor,
		hub:             hub,
		config:          cfg,
		repository:      repo,
	}
}

// WebSocket Handler
func (h *Handler) HandleWebSocket(c *gin.Context) {
	deviceID := c.Query("deviceId")
	deviceTypeStr := c.Query("deviceType")

	if deviceID == "" || deviceTypeStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId and deviceType are required"})
		return
	}

	var deviceType models.DeviceType
	switch deviceTypeStr {
	case "PSG":
		deviceType = models.DeviceTypePSG
	case "WATCH":
		deviceType = models.DeviceTypeWatch
	case "MOBILE":
		deviceType = models.DeviceTypeMobile
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deviceType, must be PSG or WATCH"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	client := ws.NewClient(h.hub, conn, deviceID, deviceType)
	h.hub.Register <- client

	// Start client pumps in goroutines
	go client.WritePump()
	go client.ReadPump()
}

// Device Handlers
func (h *Handler) GetDevices(c *gin.Context) {
	devices := h.syncService.GetConnectedDevices()
	c.JSON(http.StatusOK, devices)
}

func (h *Handler) GetDeviceHealth(c *gin.Context) {
	deviceID := c.Query("deviceId")

	if deviceID != "" {
		// Get health for specific device
		health, err := h.hub.GetDeviceHealthByID(deviceID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, health)
	} else {
		// Get health for all devices
		healthList := h.hub.GetDeviceHealth()
		c.JSON(http.StatusOK, healthList)
	}
}

// Pairing Handlers
func (h *Handler) GetPairings(c *gin.Context) {
	// Query pairings from database (persistent storage)
	persistentPairings, err := h.repository.GetAllPairings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert PersistentPairing to Pairing for response
	pairings := make([]*models.Pairing, 0, len(persistentPairings))
	for _, pp := range persistentPairings {
		pairings = append(pairings, &models.Pairing{
			PairingID: pp.PairingID,
			Device1ID: pp.Device1ID,
			Device2ID: pp.Device2ID,
			CreatedAt: pp.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, pairings)
}

func (h *Handler) CreatePairing(c *gin.Context) {
	var req models.CreatePairingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. Create in-memory pairing in Hub
	pairing, err := h.syncService.CreatePairing(req.Device1ID, req.Device2ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use request values if provided, otherwise use config defaults
	intervalSec := h.config.AutoSyncIntervalSec
	if req.AutoSyncIntervalSec != nil {
		intervalSec = *req.AutoSyncIntervalSec
	}

	sampleCount := h.config.AutoSyncSampleCount
	if req.AutoSyncSampleCount != nil {
		sampleCount = *req.AutoSyncSampleCount
	}

	intervalMs := h.config.AutoSyncIntervalMs
	if req.AutoSyncIntervalMs != nil {
		intervalMs = *req.AutoSyncIntervalMs
	}

	// 2. Save pairing to database for persistence
	persistentPairing := &models.PersistentPairing{
		PairingID:           pairing.PairingID,
		Device1ID:           pairing.Device1ID,
		Device2ID:           pairing.Device2ID,
		CreatedAt:           pairing.CreatedAt,
		AutoSyncIntervalSec: &intervalSec,
		AutoSyncSampleCount: &sampleCount,
		AutoSyncIntervalMs:  &intervalMs,
	}

	if err := h.repository.SavePairing(persistentPairing); err != nil {
		log.Printf("Failed to save pairing to DB: %v", err)
		// Don't fail the request, in-memory pairing is already created
	}

	// 3. Automatically start auto-sync with configuration
	autoSyncConfig := models.AutoSyncConfig{
		PairingID:   pairing.PairingID,
		IntervalSec: intervalSec,
		SampleCount: sampleCount,
		IntervalMs:  intervalMs,
	}

	if err := h.autoSyncMonitor.StartAutoSync(autoSyncConfig); err != nil {
		log.Printf("Warning: Failed to start auto-sync for pairing %s: %v", pairing.PairingID, err)
		// Don't fail the pairing creation, just log the warning
	} else {
		log.Printf("Auto-sync automatically started for pairing %s (interval: %ds, samples: %d, interval_ms: %dms)",
			pairing.PairingID, intervalSec, sampleCount, intervalMs)
	}

	c.JSON(http.StatusCreated, models.CreatePairingResponse{
		PairingID: pairing.PairingID,
	})
}

func (h *Handler) DeletePairing(c *gin.Context) {
	pairingID := c.Param("pairingId")

	// 1. Check if pairing exists in DB (source of truth)
	_, err := h.repository.GetPairingByID(pairingID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pairing not found"})
		return
	}

	// 2. Stop auto-sync if running
	if err := h.autoSyncMonitor.StopAutoSync(pairingID); err != nil {
		log.Printf("Note: Auto-sync was not running for pairing %s", pairingID)
		// Don't fail if auto-sync wasn't running
	} else {
		log.Printf("Auto-sync stopped for pairing %s", pairingID)
	}

	// 3. Delete from in-memory Hub (if exists, don't fail if not)
	if err := h.syncService.DeletePairing(pairingID); err != nil {
		log.Printf("Note: Pairing not in memory (devices may be disconnected): %s", pairingID)
		// Don't fail - devices might be disconnected
	}

	// 4. Delete from database (source of truth)
	if err := h.repository.DeletePairing(pairingID); err != nil {
		log.Printf("Failed to delete pairing from DB: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete pairing from database"})
		return
	}

	log.Printf("Pairing deleted: %s", pairingID)
	c.JSON(http.StatusOK, gin.H{"message": "pairing deleted"})
}

// Sync Handlers
func (h *Handler) RequestSync(c *gin.Context) {
	pairingID := c.Param("pairingId")

	record, err := h.syncService.RequestTimeSync(pairingID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SyncResponse{
		Success: true,
		Record:  record,
	})
}

func (h *Handler) GetSyncRecord(c *gin.Context) {
	recordIDStr := c.Param("recordId")

	recordID, err := strconv.ParseInt(recordIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid record ID"})
		return
	}

	record, err := h.syncService.GetSyncRecord(recordID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, record)
}

func (h *Handler) GetSyncRecords(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	deviceID := c.Query("deviceId")
	startTimeStr := c.Query("startTime")
	endTimeStr := c.Query("endTime")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit parameter"})
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset parameter"})
		return
	}

	var records []*models.TimeSyncRecord

	// Filter by device ID if provided
	if deviceID != "" {
		records, err = h.syncService.GetSyncRecordsByDevice(deviceID, limit, offset)
	} else if startTimeStr != "" && endTimeStr != "" {
		// Filter by time range if provided
		startTime, err1 := time.Parse(time.RFC3339, startTimeStr)
		endTime, err2 := time.Parse(time.RFC3339, endTimeStr)

		if err1 != nil || err2 != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time format, use RFC3339"})
			return
		}

		records, err = h.syncService.GetSyncRecordsByTimeRange(startTime, endTime, limit, offset)
	} else {
		// Get all records
		records, err = h.syncService.GetSyncRecords(limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, records)
}

// RequestMultiSync handles NTP-style multi-sampling sync request
func (h *Handler) RequestMultiSync(c *gin.Context) {
	var req models.MultiSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.syncService.RequestMultipleTimeSyncs(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.MultiSyncResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.MultiSyncResponse{
		Success: true,
		Result:  result,
	})
}

// GetAggregatedResults retrieves aggregated sync results
// Supports filtering by pairingId or time range (startTime, endTime)
func (h *Handler) GetAggregatedResults(c *gin.Context) {
	pairingID := c.Query("pairingId")
	startTimeStr := c.Query("startTime")
	endTimeStr := c.Query("endTime")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit parameter"})
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offset parameter"})
		return
	}

	var results []*models.AggregatedSyncResult

	// Filter by pairing ID if provided
	if pairingID != "" {
		results, err = h.syncService.GetAggregatedSyncResults(pairingID, limit, offset)
	} else if startTimeStr != "" && endTimeStr != "" {
		// Filter by time range if provided
		startTime, err1 := time.Parse(time.RFC3339, startTimeStr)
		endTime, err2 := time.Parse(time.RFC3339, endTimeStr)

		if err1 != nil || err2 != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time format, use RFC3339"})
			return
		}

		results, err = h.syncService.GetAggregatedSyncResultsByTimeRange(startTime, endTime, limit, offset)
	} else {
		// Get all results if no filter is provided
		results, err = h.syncService.GetAllAggregatedSyncResults(limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// GetAggregatedResult retrieves a single aggregated sync result by ID
func (h *Handler) GetAggregatedResult(c *gin.Context) {
	aggregationID := c.Param("aggregationId")

	result, err := h.syncService.GetAggregatedSyncResult(aggregationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Health Check
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

// Auto-Sync Handlers

// StartAutoSync starts automatic periodic synchronization for a pairing
func (h *Handler) StartAutoSync(c *gin.Context) {
	var req models.AutoSyncStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := models.AutoSyncConfig{
		PairingID:   req.PairingID,
		IntervalSec: req.IntervalSec,
		SampleCount: req.SampleCount,
		IntervalMs:  req.IntervalMs,
	}

	if err := h.autoSyncMonitor.StartAutoSync(config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "auto-sync started",
		"pairing_id": req.PairingID,
	})
}

// StopAutoSync stops automatic synchronization for a pairing
func (h *Handler) StopAutoSync(c *gin.Context) {
	pairingID := c.Param("pairingId")

	if err := h.autoSyncMonitor.StopAutoSync(pairingID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "auto-sync stopped",
		"pairing_id": pairingID,
	})
}

// GetAutoSyncStatus returns the status of auto-sync jobs
func (h *Handler) GetAutoSyncStatus(c *gin.Context) {
	pairingID := c.Query("pairingId")

	if pairingID != "" {
		// Get status for specific pairing
		job, err := h.autoSyncMonitor.GetStatus(pairingID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, job)
	} else {
		// Get status for all jobs
		jobs := h.autoSyncMonitor.GetAllStatuses()
		c.JSON(http.StatusOK, models.AutoSyncStatusResponse{
			Jobs: jobs,
		})
	}
}
