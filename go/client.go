package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

type Client struct {
	conn     *websocket.Conn
	WsServer *WsServer
	send     chan []byte
	rooms    map[*Room]bool
	Name     string    `json:"name"`
	ID       uuid.UUID `json:"id"`
}

func newClient(conn *websocket.Conn, wsServer *WsServer, name string) *Client {
	return &Client{
		conn:     conn,
		WsServer: wsServer,
		send:     make(chan []byte, 256),
		rooms:    make(map[*Room]bool),
		Name:     name,
		ID:       uuid.New(),
	}
}

func (client *Client) GetName() string {
	return client.Name
}

func (client *Client) handleNewMessage(jsonMessage []byte) {
	var message Message
	if err := json.Unmarshal(jsonMessage, &message); err != nil {
		log.Printf("Error on unmarsjaö JSON message %s", err)
	}

	message.Sender = client

	switch message.Action {
	case message.Action:
		roomName := message.Target

		if room := client.WsServer.findRoomByName(roomName); room != nil {
			room.broadcast <- &message
		}
	case JoinRoomAction:
		client.handleJoinRoomMessage(message)

	case LeaveRoomAction:
		client.handleLeaveRoomMessage(message)
	}
}

func ServeWs(wsServer *WsServer, w http.ResponseWriter, r *http.Request) {
	name, ok := r.URL.Query()["name"]

	if !ok || len(name[0]) < 1 {
		log.Println("Url Param 'name' is missing")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := newClient(conn, wsServer, name[0])

	go client.writePump()
	go client.readPump()

	wsServer.register <- client
	fmt.Println("New Client joined the hub!")
	fmt.Println(client)
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 10000
)

func (client *Client) readPump() {
	defer func() {
		client.disconnect()
	}()

	client.conn.SetReadLimit(maxMessageSize)
	client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, jsonMessage, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected close error: %v", err)
			}
			break
		}

		client.handleNewMessage(jsonMessage)
	}
}

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

func (client *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		err := client.conn.Close()

		if err != nil {
			log.Fatal(err)
		}
	}()

	for {
		select {
		case message, ok := <-client.send:
			err := client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(client.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-client.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		}
	}
}

func (client *Client) disconnect() {
	client.WsServer.unregister <- client

	for room := range client.rooms {
		room.unregister <- client
	}

	close(client.send)
	client.conn.Close()
}

func (client *Client) handleJoinRoomMessage(message Message) {
	roomName := message.Message

	room := client.WsServer.findRoomByName(roomName)
	if room == nil {
		room = client.WsServer.createRoom(roomName)
	}

	client.rooms[room] = true
	room.register <- client
}

func (client *Client) handleLeaveRoomMessage(message Message) {
	room := client.WsServer.findRoomByName(message.Message)
	if _, ok := client.rooms[room]; ok {
		delete(client.rooms, room)
	}

	room.unregister <- client
}
