package main_test

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	relay "github.com/quells/relay"
)

func TestRelayServer(t *testing.T) {
	hiddenListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("could not create TCP listener: %v", err)
	}
	defer hiddenListener.Close()

	hiddenServer := &http.Server{
		Handler:      relay.NewEchoServer(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	defer hiddenServer.Close()

	go func() {
		if err := hiddenServer.Serve(hiddenListener); err != http.ErrServerClosed {
			t.Fatalf("failed to listen and serve: %v", err)
		}
	}()

	hiddenURL := "ws://" + hiddenListener.Addr().String()
	log.Println(hiddenURL)

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("could not create TCP listener: %v", err)
	}
	defer l.Close()

	s := &http.Server{
		Handler:      relay.NewRelayServer(hiddenURL),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	defer s.Close()

	go func() {
		if err := s.Serve(l); err != http.ErrServerClosed {
			t.Fatalf("failed to listen and serve: %v", err)
		}
	}()

	url := "ws://" + l.Addr().String()
	log.Println(url)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("could not connect to echo server: %v", err)
	}
	defer conn.Close()

	for i := 0; i < 5; i++ {
		conn.SetWriteDeadline(time.Now().Add(time.Second))
		payload := fmt.Sprintf("%d", i)
		err := conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			t.Fatalf("could not send message: %v", err)
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("could not read message: %v", err)
		}
		response := string(msg)
		if response != payload {
			t.Fatalf("expected %s, got %s", payload, response)
		}
	}
}
