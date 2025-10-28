package api

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, handler *Handler) {
	// Health check
	// Output: {"status": "ok"}
	r.GET("/health", handler.HealthCheck)

	// WebSocket endpoint
	// Upgrade to WebSocket connection for real-time communication
	r.GET("/ws", handler.HandleWebSocket)

	// API routes
	api := r.Group("/api")
	{
		// Device management
		devices := api.Group("/devices")
		{
			// GET /api/devices
			// Output: [{"id": "psg-001", "type": "PSG", "status": "online"}, {"id": "watch-001", "type": "Watch", "status": "offline"}]
			devices.GET("", handler.GetDevices)

			// GET /api/devices/health
			// Get connection health status for all devices or specific device
			// Query params: deviceId (optional)
			// Examples:
			//   - GET /api/devices/health (all devices)
			//   - GET /api/devices/health?deviceId=psg-001 (specific device)
			// Output: {"deviceId": "psg-001", "isHealthy": true, "lastRtt": 15, "timeSinceLastPong": 5000, ...}
			devices.GET("/health", handler.GetDeviceHealth)
		}

		// Pairing management
		pairings := api.Group("/pairings")
		{
			// GET /api/pairings
			// Output: [{"id": "pair-123", "device1_id": "psg-001", "device2_id": "watch-001", "status": "active"}]
			pairings.GET("", handler.GetPairings)

			// POST /api/pairings
			// Input: {"device1Id": "psg-001", "device2Id": "watch-001"}
			// Output: {"id": "pair-123", "device1_id": "psg-001", "device2_id": "watch-001", "status": "active"}
			pairings.POST("", handler.CreatePairing)

			// DELETE /api/pairings/:pairingId
			// Example: DELETE /api/pairings/pair-123
			// Output: {"message": "Pairing deleted successfully"}
			pairings.DELETE("/:pairingId", handler.DeletePairing)
		}

		// Time synchronization
		sync := api.Group("/sync")
		{
			// POST /api/sync/:pairingId
			// Single time synchronization request
			// Example: POST /api/sync/pair-123
			// Output: {"success": true, "record": {...}}
			sync.POST("/:pairingId", handler.RequestSync)

			// POST /api/sync/multi
			// NTP-style multi-sampling synchronization
			// Input: {"pairing_id": "pair-123", "sample_count": 10, "interval_ms": 200}
			// Output: {"success": true, "result": {"best_offset": -150, "confidence": 0.94, ...}}
			sync.POST("/multi", handler.RequestMultiSync)

			// GET /api/sync/records
			// Get individual sync records
			// Output: [{"id": 1, "device1_id": "psg-001", "time_difference": -150, ...}]
			sync.GET("/records", handler.GetSyncRecords)

			// GET /api/sync/records/:recordId
			// Get a single sync record by ID
			// Example: GET /api/sync/records/123
			// Output: {"id": 123, "device1_id": "psg-001", "time_difference": -150, ...}
			sync.GET("/records/:recordId", handler.GetSyncRecord)

			// GET /api/sync/aggregated
			// Get aggregated NTP results with optional filters
			// Query params:
			//   - pairingId (optional): Filter by specific pairing
			//   - startTime, endTime (optional): Filter by time range (RFC3339 format)
			//   - limit, offset: Pagination
			// Examples:
			//   - GET /api/sync/aggregated?pairingId=pair-123&limit=10
			//   - GET /api/sync/aggregated?startTime=2024-01-01T00:00:00Z&endTime=2024-01-31T23:59:59Z
			//   - GET /api/sync/aggregated (all results)
			// Output: [{"aggregation_id": "agg-123", "best_offset": -150, "confidence": 0.94, ...}]
			sync.GET("/aggregated", handler.GetAggregatedResults)

			// GET /api/sync/aggregated/:aggregationId
			// Get a single aggregated result with all measurements
			// Output: {"aggregation_id": "agg-123", "measurements": [...], ...}
			sync.GET("/aggregated/:aggregationId", handler.GetAggregatedResult)
		}

		// Auto-Sync management
		autoSync := api.Group("/auto-sync")
		{
			// POST /api/auto-sync/start
			// Start automatic periodic synchronization for a pairing
			// Input: {"pairing_id": "pair-123", "interval_sec": 60, "sample_count": 8, "interval_ms": 200}
			// Output: {"message": "auto-sync started", "pairing_id": "pair-123"}
			autoSync.POST("/start", handler.StartAutoSync)

			// POST /api/auto-sync/stop/:pairingId
			// Stop automatic synchronization for a pairing
			// Example: POST /api/auto-sync/stop/pair-123
			// Output: {"message": "auto-sync stopped", "pairing_id": "pair-123"}
			autoSync.POST("/stop/:pairingId", handler.StopAutoSync)

			// GET /api/auto-sync/status
			// Get status of all auto-sync jobs or specific pairing
			// Query params: pairingId (optional)
			// Examples:
			//   - GET /api/auto-sync/status (all jobs)
			//   - GET /api/auto-sync/status?pairingId=pair-123 (specific job)
			// Output: {"jobs": [{"pairing_id": "pair-123", "status": "RUNNING", ...}]}
			autoSync.GET("/status", handler.GetAutoSyncStatus)
		}
	}
}
