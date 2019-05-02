package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// RelayServer inserts a MITM for WebSockets
type RelayServer struct {
	internalURL string
}

// NewRelayServer is a constructor for RelayServer
func NewRelayServer(internalURL string) RelayServer {
	return RelayServer{
		internalURL,
	}
}

func (s RelayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	externalConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade external connection: %v", err)
		http.Error(w, "could not upgrade to websocket", http.StatusBadRequest)
		return
	}
	log.Println("upgraded external connection")

	internalConn, _, err := websocket.DefaultDialer.Dial(s.internalURL, nil)
	if err != nil {
		log.Printf("failed to upgrade internal connection: %v", err)
		http.Error(w, "could not upgrade to websocket", http.StatusBadRequest)
		return
	}
	log.Println("made internal connection")

	client := NewRelayClient(externalConn, internalConn)
	go client.readExternalPump()
	go client.readInternalPump()
	go client.writePump()
}

// RelayClient is a MITM for WebSockets
type RelayClient struct {
	external, internal *websocket.Conn
	inbound, outbound  chan []byte
}

// NewRelayClient is a constructor for RelayClient
func NewRelayClient(external, internal *websocket.Conn) RelayClient {
	return RelayClient{
		external: external,
		internal: internal,
		inbound:  make(chan []byte),
		outbound: make(chan []byte),
	}
}

func (c *RelayClient) readExternalPump() {
	defer func() {
		log.Println("exit readExternalPump")
		c.external.Close()
	}()

	c.external.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.external.SetPongHandler(func(string) error {
		c.external.SetReadDeadline(time.Now().Add(10 * time.Second))
		return nil
	})

	for {
		waitStart := time.Now()
		_, msg, err := c.external.ReadMessage()
		log.Printf("spent %s waiting to read inbound msg", time.Since(waitStart))
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected error: %v", err)
			}
			break
		}
		log.Printf("inbound %v", msg)

		c.inbound <- msg
		log.Println("done reading inbound")
	}
}

func (c *RelayClient) readInternalPump() {
	defer func() {
		log.Println("exit readInternalPump")
		c.internal.Close()
	}()

	c.internal.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.internal.SetPongHandler(func(string) error {
		c.internal.SetReadDeadline(time.Now().Add(10 * time.Second))
		return nil
	})

	for {
		waitStart := time.Now()
		_, msg, err := c.internal.ReadMessage()
		log.Printf("spent %s waiting to read outbound msg", time.Since(waitStart))
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected error: %v", err)
			}
			break
		}
		log.Printf("outbound %v", msg)

		c.outbound <- msg
		log.Println("done reading outbound")
	}
}

func (c *RelayClient) writePump() {
	externalTicker := time.NewTicker(60 * time.Second)
	internalTicker := time.NewTicker(60 * time.Second)
	defer func() {
		log.Println("exit writePump")
		externalTicker.Stop()
		c.external.Close()
		c.internal.Close()
	}()

	for {
		waitStart := time.Now()
		select {
		case msg, ok := <-c.outbound:
			log.Printf("spent %s waiting to write outbound message", time.Since(waitStart))
			c.external.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.external.WriteMessage(websocket.CloseMessage, []byte{})
			}

			err := c.external.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				return
			}
			log.Printf("wrote external %v", msg)

		case msg, ok := <-c.inbound:
			log.Printf("spent %s waiting to write inbound message", time.Since(waitStart))
			c.internal.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.internal.WriteMessage(websocket.CloseMessage, []byte{})
			}

			err := c.internal.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				return
			}
			log.Printf("wrote internal %v", msg)

		case <-externalTicker.C:
			c.external.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.external.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			log.Println("wrote external ping")

		case <-internalTicker.C:
			c.internal.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.internal.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			log.Println("wrote internal ping")
		}
	}
}
