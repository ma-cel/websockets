package main

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"log"
	"websockets/config"
)

var ctx = context.Background()

type Room struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Private    bool      `json:"private"`
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan *Message
}

func NewRoom(name string, private bool) *Room {
	return &Room{
		ID:         uuid.New(),
		Name:       name,
		Private:    private,
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *Message),
	}
}

const welcomeMessage = "%s joined the room"
const leftMessage = "%s left the room"

func (room *Room) GetId() string {
	return room.ID.String()
}

func (room *Room) RunRoom() {
	go room.subscribeToRoomMessages()

	for {
		select {
		case client := <-room.register:
			room.registerClientInRoom(client)

		case client := <-room.unregister:
			room.unregisterClientInRoom(client)

		case message := <-room.broadcast:
			room.publishRoomMessage(message.encode())
		}
	}
}

func (room *Room) GetName() string {
	return room.Name
}

func (room *Room) GetPrivate() bool {
	return room.Private
}

func (room *Room) registerClientInRoom(client *Client) {
	if !room.Private {
		room.notifyClientJoined(client)
	}
	room.clients[client] = true
}

func (room *Room) unregisterClientInRoom(client *Client) {
	room.notifyClientLeft(client)
	room.clients[client] = true
}

func (room *Room) broadcastToClientsInRoom(message []byte) {
	for client := range room.clients {
		client.send <- message
	}
}

func (room *Room) notifyClientJoined(client *Client) {
	message := &Message{
		Action:  SendMessageAction,
		Target:  room,
		Message: fmt.Sprintf(welcomeMessage, client.GetName()),
	}

	room.broadcastToClientsInRoom(message.encode())
	room.publishRoomMessage(message.encode())
}

func (room *Room) notifyClientLeft(client *Client) {
	message := &Message{
		Action:  LeaveRoomAction,
		Target:  room,
		Message: fmt.Sprintf(leftMessage, client.GetName()),
	}

	room.broadcastToClientsInRoom(message.encode())
}

func (room *Room) publishRoomMessage(message []byte) {
	err := config.Redis.Publish(ctx, room.GetName(), message).Err()

	if err != nil {
		log.Println(err)
	}
}

func (room *Room) subscribeToRoomMessages() {
	pubsub := config.Redis.Subscribe(ctx, room.GetName())

	ch := pubsub.Channel()

	for msg := range ch {
		room.broadcastToClientsInRoom([]byte(msg.Payload))
	}
}
