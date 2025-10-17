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

			// GET /api/sync/aggregated
			// Get aggregated NTP results by pairing
			// Query params: pairingId (required), limit, offset
			// Output: [{"aggregation_id": "agg-123", "best_offset": -150, "confidence": 0.94, ...}]
			sync.GET("/aggregated", handler.GetAggregatedResults)

			// GET /api/sync/aggregated/:aggregationId
			// Get a single aggregated result with all measurements
			// Output: {"aggregation_id": "agg-123", "measurements": [...], ...}
			sync.GET("/aggregated/:aggregationId", handler.GetAggregatedResult)
		}
	}
}
