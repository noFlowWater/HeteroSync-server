package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

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
	syncService *service.SyncService
	hub         *ws.Hub
}

func NewHandler(syncService *service.SyncService, hub *ws.Hub) *Handler {
	return &Handler{
		syncService: syncService,
		hub:         hub,
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
	pairings := h.syncService.GetPairings()
	c.JSON(http.StatusOK, pairings)
}

func (h *Handler) CreatePairing(c *gin.Context) {
	var req models.CreatePairingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pairing, err := h.syncService.CreatePairing(req.Device1ID, req.Device2ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, models.CreatePairingResponse{
		PairingID: pairing.PairingID,
	})
}

func (h *Handler) DeletePairing(c *gin.Context) {
	pairingID := c.Param("pairingId")

	if err := h.syncService.DeletePairing(pairingID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

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
