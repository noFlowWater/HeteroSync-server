package websocket

import (
	"encoding/json"
	"log"
	"time"

	"time-sync-server/internal/models"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Application-level PING period (20 seconds as recommended)
	appPingPeriod = 40 * time.Second

	// Maximum message size allowed from peer
	maxMessageSize = 512 // Increased to 8KB to handle longer JSON messages
)

type Client struct {
	Hub          *Hub
	Conn         *websocket.Conn
	Send         chan []byte
	DeviceID     string
	DeviceType   models.DeviceType
	ConnectedAt  time.Time // Connection establishment time
	LastPingSent time.Time // Last application-level PING sent time
	LastPongRecv time.Time // Last application-level PONG received time
	LastRTT      int64     // Last measured RTT in milliseconds
}

func NewClient(hub *Hub, conn *websocket.Conn, deviceID string, deviceType models.DeviceType) *Client {
	now := time.Now()
	return &Client{
		Hub:          hub,
		Conn:         conn,
		Send:         make(chan []byte, 256),
		DeviceID:     deviceID,
		DeviceType:   deviceType,
		ConnectedAt:  now,
		LastPingSent: now,
		LastPongRecv: now,
		LastRTT:      0,
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket error: %v", err)
			}
			break
		}

		// Handle incoming message
		c.Hub.HandleMessage(c, message)
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) WritePump() {
	protocolPingTicker := time.NewTicker(pingPeriod)
	appPingTicker := time.NewTicker(appPingPeriod)
	defer func() {
		protocolPingTicker.Stop()
		appPingTicker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-protocolPingTicker.C:
			// WebSocket protocol-level ping
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-appPingTicker.C:
			// Application-level PING message
			c.sendAppPing()
		}
	}
}

// SendMessage sends a JSON message to the client
func (c *Client) SendMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.Send <- data:
		return nil
	default:
		return websocket.ErrCloseSent
	}
}

// sendAppPing sends an application-level PING message to the client
func (c *Client) sendAppPing() {
	c.LastPingSent = time.Now()
	pingMsg := models.PingMessage{
		Type:      models.MessageTypePing,
		Timestamp: c.LastPingSent.UnixMilli(),
	}

	if err := c.SendMessage(pingMsg); err != nil {
		log.Printf("Failed to send PING to device %s: %v", c.DeviceID, err)
		// 에러는 중요하므로 유지
	}
}
