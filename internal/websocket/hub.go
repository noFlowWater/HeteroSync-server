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

	mu sync.RWMutex
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

func (h *Hub) GetConnectedDevices() []*models.Device {
	h.mu.RLock()
	defer h.mu.RUnlock()

	devices := make([]*models.Device, 0, len(h.Clients))
	for _, client := range h.Clients {
		devices = append(devices, &models.Device{
			DeviceID:   client.DeviceID,
			DeviceType: client.DeviceType,
			ConnectedAt: time.Now(), // We don't track connection time, so use current time
		})
	}
	return devices
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
	var baseMsg models.WSMessage
	if err := json.Unmarshal(message, &baseMsg); err != nil {
		log.Printf("Failed to unmarshal message: %v", err)
		return
	}

	switch baseMsg.Type {
	case models.MessageTypeTimeResponse:
		var timeResp models.TimeResponseMessage
		if err := json.Unmarshal(message, &timeResp); err != nil {
			log.Printf("Failed to unmarshal time response: %v", err)
			return
		}
		h.handleTimeResponse(client, &timeResp)

	default:
		log.Printf("Unknown message type: %s", baseMsg.Type)
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
