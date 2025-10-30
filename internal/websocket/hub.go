package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"time-sync-server/internal/models"
)

type Hub struct {
	// Registered clients (deviceID -> Client)
	Clients map[string]*Client

	// Active pairings (pairingID -> Pairing)
	Pairings map[string]*models.Pairing

	// Pending time sync requests (requestID -> PendingRequest)
	PendingRequests map[string]*PendingRequest

	// Register requests from the clients
	Register chan *Client

	// Unregister requests from clients
	Unregister chan *Client

	// Pairing operator (set after initialization to avoid circular dependency)
	pairingOperator PairingOperator

	mu sync.RWMutex
}

// PairingOperator interface to avoid circular dependency
type PairingOperator interface {
	OnDeviceConnected(deviceID string)
}

type PendingRequest struct {
	RequestID          string
	PairingID          string
	Device1ID          string
	Device2ID          string
	Device1Response    *int64
	Device2Response    *int64
	ServerRequestTime  int64
	// RTT measurement fields
	Device1SendTime    int64  // Device1 request send time (microseconds)
	Device2SendTime    int64  // Device2 request send time (microseconds)
	Device1ReceiveTime *int64 // Device1 response receive time (microseconds)
	Device2ReceiveTime *int64 // Device2 response receive time (microseconds)
	ResponseChan       chan *models.TimeSyncRecord
	TimeoutTimer       *time.Timer
}

func NewHub() *Hub {
	return &Hub{
		Clients:         make(map[string]*Client),
		Pairings:        make(map[string]*models.Pairing),
		PendingRequests: make(map[string]*PendingRequest),
		Register:        make(chan *Client),
		Unregister:      make(chan *Client),
	}
}

func (h *Hub) Run() {
	// Start dead connection detector
	go h.detectDeadConnections()

	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client.DeviceID] = client
			h.mu.Unlock()
			log.Printf("Client registered: %s (%s)", client.DeviceID, client.DeviceType)

			// Send connected message
			msg := models.ConnectedMessage{
				Type:       models.MessageTypeConnected,
				DeviceID:   client.DeviceID,
				ServerTime: time.Now().UnixMilli(),
			}
			client.SendMessage(msg)

			// Trigger pairing restoration (run in goroutine to avoid blocking)
			if h.pairingOperator != nil {
				go h.pairingOperator.OnDeviceConnected(client.DeviceID)
			}

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client.DeviceID]; ok {
				delete(h.Clients, client.DeviceID)
				close(client.Send)
				log.Printf("Client unregistered: %s", client.DeviceID)

				// Remove pairings involving this device
				for pairingID, pairing := range h.Pairings {
					if pairing.Device1ID == client.DeviceID || pairing.Device2ID == client.DeviceID {
						delete(h.Pairings, pairingID)
						log.Printf("Pairing removed: %s", pairingID)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// detectDeadConnections periodically checks for and closes dead connections
func (h *Hub) detectDeadConnections() {
	// Check interval: every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Timeout threshold: no PONG for 120 seconds
	const deadConnectionTimeout = 120 * time.Second

	for range ticker.C {
		h.mu.Lock()
		now := time.Now()
		deadClients := make([]*Client, 0)

		for _, client := range h.Clients {
			timeSinceLastPong := now.Sub(client.LastPongRecv)
			if timeSinceLastPong > deadConnectionTimeout {
				log.Printf("Dead connection detected: %s (no PONG for %v)",
					client.DeviceID, timeSinceLastPong)
				deadClients = append(deadClients, client)
			}
		}
		h.mu.Unlock()

		// Unregister dead clients
		for _, client := range deadClients {
			// Close the connection
			client.Conn.Close()
			// This will trigger the client's ReadPump to exit and send Unregister
		}
	}
}

func (h *Hub) GetConnectedDevices() []*models.Device {
	h.mu.RLock()
	defer h.mu.RUnlock()

	devices := make([]*models.Device, 0, len(h.Clients))
	for _, client := range h.Clients {
		devices = append(devices, &models.Device{
			DeviceID:    client.DeviceID,
			DeviceType:  client.DeviceType,
			ConnectedAt: client.ConnectedAt,
		})
	}
	return devices
}

// GetDeviceHealth retrieves health information for all connected devices
func (h *Hub) GetDeviceHealth() []*models.DeviceHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()

	const healthThreshold = 90 * time.Second // Consider unhealthy if no PONG for 90 seconds

	healthList := make([]*models.DeviceHealth, 0, len(h.Clients))
	now := time.Now()

	for _, client := range h.Clients {
		timeSinceLastPong := now.Sub(client.LastPongRecv).Milliseconds()
		isHealthy := now.Sub(client.LastPongRecv) < healthThreshold

		healthList = append(healthList, &models.DeviceHealth{
			DeviceID:          client.DeviceID,
			DeviceType:        client.DeviceType,
			ConnectedAt:       client.ConnectedAt,
			LastPingSent:      client.LastPingSent,
			LastPongRecv:      client.LastPongRecv,
			LastRTT:           client.LastRTT,
			IsHealthy:         isHealthy,
			TimeSinceLastPong: timeSinceLastPong,
		})
	}
	return healthList
}

// GetDeviceHealthByID retrieves health information for a specific device
func (h *Hub) GetDeviceHealthByID(deviceID string) (*models.DeviceHealth, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	client, ok := h.Clients[deviceID]
	if !ok {
		return nil, &DeviceNotConnectedError{DeviceID: deviceID}
	}

	const healthThreshold = 90 * time.Second
	now := time.Now()
	timeSinceLastPong := now.Sub(client.LastPongRecv).Milliseconds()
	isHealthy := now.Sub(client.LastPongRecv) < healthThreshold

	return &models.DeviceHealth{
		DeviceID:          client.DeviceID,
		DeviceType:        client.DeviceType,
		ConnectedAt:       client.ConnectedAt,
		LastPingSent:      client.LastPingSent,
		LastPongRecv:      client.LastPongRecv,
		LastRTT:           client.LastRTT,
		IsHealthy:         isHealthy,
		TimeSinceLastPong: timeSinceLastPong,
	}, nil
}

func (h *Hub) GetPairings() []*models.Pairing {
	h.mu.RLock()
	defer h.mu.RUnlock()

	pairings := make([]*models.Pairing, 0, len(h.Pairings))
	for _, pairing := range h.Pairings {
		pairings = append(pairings, pairing)
	}
	return pairings
}

func (h *Hub) CreatePairing(device1ID, device2ID string) (*models.Pairing, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if both devices are connected
	if _, ok := h.Clients[device1ID]; !ok {
		return nil, &DeviceNotConnectedError{DeviceID: device1ID}
	}
	if _, ok := h.Clients[device2ID]; !ok {
		return nil, &DeviceNotConnectedError{DeviceID: device2ID}
	}

	pairing := &models.Pairing{
		PairingID: uuid.New().String(),
		Device1ID: device1ID,
		Device2ID: device2ID,
		CreatedAt: time.Now(),
	}

	h.Pairings[pairing.PairingID] = pairing
	log.Printf("Pairing created: %s (%s <-> %s)", pairing.PairingID, device1ID, device2ID)

	return pairing, nil
}

func (h *Hub) DeletePairing(pairingID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.Pairings[pairingID]; !ok {
		return &PairingNotFoundError{PairingID: pairingID}
	}

	delete(h.Pairings, pairingID)
	log.Printf("Pairing deleted: %s", pairingID)
	return nil
}

func (h *Hub) RequestTimeSync(pairingID string, timeout time.Duration) (*models.TimeSyncRecord, error) {
	h.mu.RLock()
	pairing, ok := h.Pairings[pairingID]
	if !ok {
		h.mu.RUnlock()
		return nil, &PairingNotFoundError{PairingID: pairingID}
	}

	client1, ok1 := h.Clients[pairing.Device1ID]
	client2, ok2 := h.Clients[pairing.Device2ID]
	h.mu.RUnlock()

	if !ok1 {
		return nil, &DeviceNotConnectedError{DeviceID: pairing.Device1ID}
	}
	if !ok2 {
		return nil, &DeviceNotConnectedError{DeviceID: pairing.Device2ID}
	}

	requestID := uuid.New().String()
	serverRequestTime := time.Now().UnixMilli()

	responseChan := make(chan *models.TimeSyncRecord, 1)

	pendingReq := &PendingRequest{
		RequestID:         requestID,
		PairingID:         pairingID,
		Device1ID:         pairing.Device1ID,
		Device2ID:         pairing.Device2ID,
		ServerRequestTime: serverRequestTime,
		ResponseChan:      responseChan,
	}

	h.mu.Lock()
	h.PendingRequests[requestID] = pendingReq
	h.mu.Unlock()

	// Set timeout
	pendingReq.TimeoutTimer = time.AfterFunc(timeout, func() {
		h.handleTimeout(requestID)
	})

	// Send time request to both devices
	timeReqMsg := models.TimeRequestMessage{
		Type:      models.MessageTypeTimeRequest,
		RequestID: requestID,
		PairingID: pairingID,
	}

	// Send time request to both devices simultaneously
	go func() {
		// RTT START: Record send time for Device1
		sendTime := time.Now().UnixMicro()
		h.mu.Lock()
		if req, ok := h.PendingRequests[requestID]; ok {
			req.Device1SendTime = sendTime
		}
		h.mu.Unlock()

		if err := client1.SendMessage(timeReqMsg); err != nil {
			log.Printf("Failed to send time request to device %s: %v", client1.DeviceID, err)
		}
	}()
	go func() {
		// RTT START: Record send time for Device2
		sendTime := time.Now().UnixMicro()
		h.mu.Lock()
		if req, ok := h.PendingRequests[requestID]; ok {
			req.Device2SendTime = sendTime
		}
		h.mu.Unlock()

		if err := client2.SendMessage(timeReqMsg); err != nil {
			log.Printf("Failed to send time request to device %s: %v", client2.DeviceID, err)
		}
	}()

	// Wait for response or timeout
	record := <-responseChan
	return record, nil
}

func (h *Hub) HandleMessage(client *Client, message []byte) {
	// Debug: Log the raw message
	log.Printf("Received raw message from %s: %s", client.DeviceID, string(message))
	
	var baseMsg models.WSMessage
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		log.Printf("Failed to unmarshal message from %s: %v, raw: %s", client.DeviceID, err, string(message))
		return
	}

	log.Printf("Parsed message type from %s: %s", client.DeviceID, baseMsg.Type)

	switch baseMsg.Type {
	case models.MessageTypeTimeResponse:
		var timeResp models.TimeResponseMessage
		if err := json.Unmarshal(message, &timeResp); err != nil {
			log.Printf("Failed to unmarshal time response: %v", err)
			return
		}
		h.handleTimeResponse(client, &timeResp)

	case models.MessageTypePing:
		var pingMsg models.PingMessage
		if err := json.Unmarshal(message, &pingMsg); err != nil {
			log.Printf("Failed to unmarshal PING message: %v", err)
			return
		}
		h.handlePing(client, &pingMsg)

	case models.MessageTypePong:
		var pongMsg models.PongMessage
		if err := json.Unmarshal(message, &pongMsg); err != nil {
			log.Printf("Failed to unmarshal PONG message: %v", err)
			return
		}
		h.handlePong(client, &pongMsg)

	default:
		log.Printf("Unknown message type: '%s' from client %s", baseMsg.Type, client.DeviceID)
	}
}

func (h *Hub) handleTimeResponse(client *Client, resp *models.TimeResponseMessage) {
	// RTT END: Record receive time before acquiring lock
	receiveTime := time.Now().UnixMicro()

	h.mu.Lock()
	defer h.mu.Unlock()

	pendingReq, ok := h.PendingRequests[resp.RequestID]
	if !ok {
		log.Printf("No pending request found for requestID: %s", resp.RequestID)
		return
	}

	// Store response based on device
	if client.DeviceID == pendingReq.Device1ID {
		pendingReq.Device1Response = &resp.Timestamp
		pendingReq.Device1ReceiveTime = &receiveTime
	} else if client.DeviceID == pendingReq.Device2ID {
		pendingReq.Device2Response = &resp.Timestamp
		pendingReq.Device2ReceiveTime = &receiveTime
	} else {
		log.Printf("Response from unexpected device: %s", client.DeviceID)
		return
	}

	// Check if we have both responses
	if pendingReq.Device1Response != nil && pendingReq.Device2Response != nil {
		h.completeSyncRequest(pendingReq)
	}
}

func (h *Hub) handleTimeout(requestID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	pendingReq, ok := h.PendingRequests[requestID]
	if !ok {
		return
	}

	log.Printf("Time sync request timeout: %s", requestID)
	h.completeSyncRequest(pendingReq)
}

func (h *Hub) completeSyncRequest(pendingReq *PendingRequest) {
	// Stop timeout timer
	if pendingReq.TimeoutTimer != nil {
		pendingReq.TimeoutTimer.Stop()
	}

	serverResponseTime := time.Now().UnixMilli()

	// Determine status
	var status models.SyncStatus
	var errorMsg *string
	if pendingReq.Device1Response != nil && pendingReq.Device2Response != nil {
		status = models.SyncStatusSuccess
	} else if pendingReq.Device1Response != nil || pendingReq.Device2Response != nil {
		status = models.SyncStatusPartial
		msg := "One or more devices did not respond"
		errorMsg = &msg
	} else {
		status = models.SyncStatusFailed
		msg := "Both devices failed to respond"
		errorMsg = &msg
	}

	// Get device types
	var device1Type, device2Type models.DeviceType
	if client1, ok := h.Clients[pendingReq.Device1ID]; ok {
		device1Type = client1.DeviceType
	}
	if client2, ok := h.Clients[pendingReq.Device2ID]; ok {
		device2Type = client2.DeviceType
	}

	// Calculate RTT for each device
	var device1RTT, device2RTT *int64
	if pendingReq.Device1ReceiveTime != nil && pendingReq.Device1SendTime > 0 {
		rtt := *pendingReq.Device1ReceiveTime - pendingReq.Device1SendTime
		device1RTT = &rtt
	}
	if pendingReq.Device2ReceiveTime != nil && pendingReq.Device2SendTime > 0 {
		rtt := *pendingReq.Device2ReceiveTime - pendingReq.Device2SendTime
		device2RTT = &rtt
	}

	// Calculate RAW time difference (no network compensation)
	// Network delay compensation will be applied by NTPSelector during multi-sampling
	var timeDifference *int64
	if pendingReq.Device1Response != nil && pendingReq.Device2Response != nil {
		// Store raw difference: Device1Time - Device2Time
		// Negative = Device1 is behind Device2
		// Positive = Device1 is ahead of Device2
		rawDiff := *pendingReq.Device1Response - *pendingReq.Device2Response
		timeDifference = &rawDiff
	}

	record := &models.TimeSyncRecord{
		Device1ID:          pendingReq.Device1ID,
		Device1Type:        device1Type,
		Device1Timestamp:   pendingReq.Device1Response,
		Device2ID:          pendingReq.Device2ID,
		Device2Type:        device2Type,
		Device2Timestamp:   pendingReq.Device2Response,
		ServerRequestTime:  pendingReq.ServerRequestTime,
		ServerResponseTime: &serverResponseTime,
		Device1RTT:         device1RTT,
		Device2RTT:         device2RTT,
		TimeDifference:     timeDifference,
		Status:             status,
		ErrorMessage:       errorMsg,
		CreatedAt:          time.Now().UnixMilli(),
	}

	// Send result through channel
	select {
	case pendingReq.ResponseChan <- record:
	default:
	}

	// Clean up
	delete(h.PendingRequests, pendingReq.RequestID)
}

// handlePing handles incoming PING messages from clients and responds with PONG
func (h *Hub) handlePing(client *Client, ping *models.PingMessage) {
	// log.Printf("Received PING from client %s at %d", client.DeviceID, ping.Timestamp)

	// Send PONG response
	pongMsg := models.PongMessage{
		Type:      models.MessageTypePong,
		Timestamp: time.Now().UnixMilli(),
	}

	if err := client.SendMessage(pongMsg); err != nil {
		log.Printf("Failed to send PONG to device %s: %v", client.DeviceID, err)
	}
}

// handlePong handles incoming PONG messages from clients
func (h *Hub) handlePong(client *Client, pong *models.PongMessage) {
	now := time.Now()
	client.LastPongRecv = now

	// Calculate RTT if we have a recent ping
	var rtt int64
	if !client.LastPingSent.IsZero() {
		rtt = now.Sub(client.LastPingSent).Milliseconds()
		client.LastRTT = rtt
	}

	// log.Printf("Received PONG from client %s, RTT: %dms", client.DeviceID, rtt)
}

// SetPairingOperator sets the pairing operator (called after initialization to avoid circular dependency)
func (h *Hub) SetPairingOperator(operator PairingOperator) {
	h.pairingOperator = operator
}

// IsDeviceConnected checks if a device is currently connected
func (h *Hub) IsDeviceConnected(deviceID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.Clients[deviceID]
	return ok
}

// IsPairingRestored checks if a pairing is already restored in memory
func (h *Hub) IsPairingRestored(pairingID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.Pairings[pairingID]
	return ok
}

// RestorePairing restores a pairing from DB to in-memory
func (h *Hub) RestorePairing(pairing *models.Pairing) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if already exists
	if _, ok := h.Pairings[pairing.PairingID]; ok {
		return nil // Already restored, no error
	}

	// Check if both devices are connected
	if _, ok := h.Clients[pairing.Device1ID]; !ok {
		return &DeviceNotConnectedError{DeviceID: pairing.Device1ID}
	}
	if _, ok := h.Clients[pairing.Device2ID]; !ok {
		return &DeviceNotConnectedError{DeviceID: pairing.Device2ID}
	}

	h.Pairings[pairing.PairingID] = pairing
	return nil
}

// Custom errors
type DeviceNotConnectedError struct {
	DeviceID string
}

func (e *DeviceNotConnectedError) Error() string {
	return "device not connected: " + e.DeviceID
}

type PairingNotFoundError struct {
	PairingID string
}

func (e *PairingNotFoundError) Error() string {
	return "pairing not found: " + e.PairingID
}
