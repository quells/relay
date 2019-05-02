package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// EchoServer implements a WebSocket echo server
type EchoServer struct{}

// NewEchoServer is a constructor for EchoServer
func NewEchoServer() EchoServer {
	return EchoServer{}
}

func (s EchoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade connection: %v", err)
		http.Error(w, "could not upgrade to websocket", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("read error: %v", err)
			}
			break
		}
		log.Printf("echo received %v", msg)

		conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err = conn.WriteMessage(msgType, msg)
		if err != nil {
			return
		}
		log.Printf("echo sent %v", msg)
	}
}
