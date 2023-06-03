package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"log"
	"websockets/config"
	"websockets/models"
)

const PubSubGeneralChannel = "general"

type WsServer struct {
	clients        map[*Client]bool
	register       chan *Client
	unregister     chan *Client
	broadcast      chan []byte
	rooms          map[*Room]bool
	users          []models.User
	roomRepository models.RoomRepository
	userRepository models.UserRepository
}

func NewWebsocketServer(roomRepository models.RoomRepository, userRepository models.UserRepository) *WsServer {
	wsServer := &WsServer{
		clients:        make(map[*Client]bool),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		broadcast:      make(chan []byte),
		rooms:          make(map[*Room]bool),
		roomRepository: roomRepository,
		userRepository: userRepository,
	}

	wsServer.users = userRepository.GetAllUsers()

	return wsServer
}

func (server *WsServer) Run() {
	go server.listenPuSubChannel()
	for {
		select {
		case client := <-server.register:
			server.registerClient(client)

		case client := <-server.unregister:
			server.unregisterClient(client)

		case message := <-server.broadcast:
			server.broadcastToClients(message)
		}
	}
}

func (server *WsServer) broadcastToClients(message []byte) {
	for client := range server.clients {
		client.send <- message
	}
}

func (server *WsServer) registerClient(client *Client) {
	server.userRepository.AddUser(client)
	server.publishClientJoined(client)
	server.listOnlineClients(client)
	server.clients[client] = true
}

func (server *WsServer) unregisterClient(client *Client) {
	if _, ok := server.clients[client]; ok {
		delete(server.clients, client)
		server.userRepository.RemoveUser(client)
		server.publishClientLeft(client)
	}
}

func (server *WsServer) findRoomByName(name string) *Room {
	var foundRoom *Room
	for room := range server.rooms {
		if room.GetName() == name {
			foundRoom = room
			break
		}
	}

	if foundRoom == nil {
		foundRoom = server.runRoomFromRepository(name)
	}

	return foundRoom
}

func (server *WsServer) findRoomById(ID string) *Room {
	var foundRoom *Room
	for room := range server.rooms {
		if room.GetId() == ID {
			foundRoom = room
			break
		}
	}

	return foundRoom
}

func (server *WsServer) runRoomFromRepository(name string) *Room {
	var room *Room
	dbRoom := server.roomRepository.FindRoomByName(name)
	if dbRoom != nil {
		room = NewRoom(dbRoom.GetName(), dbRoom.GetPrivate())
		room.ID, _ = uuid.Parse(dbRoom.GetId())

		go room.RunRoom()
		server.rooms[room] = true
	}
	return room
}

func (server *WsServer) createRoom(name string, private bool) *Room {
	room := NewRoom(name, private)
	server.roomRepository.AddRoom(room)

	go room.RunRoom()
	server.rooms[room] = true

	return room
}

func (server *WsServer) findClientByID(ID string) *Client {
	var foundClient *Client
	for client := range server.clients {
		if client.ID.String() == ID {
			foundClient = client
			break
		}
	}
	return foundClient
}

func (server *WsServer) notifyClientJoined(client *Client) {
	message := &Message{
		Action: UserJoinedAction,
		Sender: client,
	}

	server.broadcastToClients(message.encode())
}

func (server *WsServer) notifyClientLeft(client *Client) {
	message := &Message{
		Action: UserLeftAction,
		Sender: client,
	}

	server.broadcastToClients(message.encode())
}

func (server *WsServer) listOnlineClients(client *Client) {
	for _, user := range server.users {
		message := &Message{
			Action: UserJoinedAction,
			Sender: user,
		}
		client.send <- message.encode()
	}
}

func (server *WsServer) publishClientJoined(client *Client) {

	message := &Message{
		Action: UserJoinedAction,
		Sender: client,
	}

	if err := config.Redis.Publish(ctx, PubSubGeneralChannel, message.encode()).Err(); err != nil {
		log.Println(err)
	}

}

func (server *WsServer) publishClientLeft(client *Client) {
	message := Message{
		Action: UserLeftAction,
		Sender: client,
	}

	if err := config.Redis.Publish(ctx, PubSubGeneralChannel, message.encode()).Err(); err != nil {
		log.Println(err)
	}
}

func (server *WsServer) listenPuSubChannel() {
	pubSub := config.Redis.Subscribe(ctx, PubSubGeneralChannel)
	ch := pubSub.Channel()

	for msg := range ch {
		var message Message
		if err := json.Unmarshal([]byte(msg.Payload), &message); err != nil {
			log.Printf("Error on unmarshal JSON message %s", err)
			return
		}

		switch message.Action {
		case UserJoinedAction:
			server.handleUserJoined(message)
		case UserLeftAction:
			server.handleUserLeft(message)
		case JoinRoomPrivateAction:
			server.handleUserJoinedPrivate(message)
		}
	}
}

func (server *WsServer) handleUserJoined(message Message) {
	server.users = append(server.users, message.Sender)
	server.broadcastToClients(message.encode())
}

func (server *WsServer) handleUserLeft(message Message) {

	for i, user := range server.users {
		if user.GetId() == message.Sender.GetId() {
			server.users[i] = server.users[len(server.users)-1]
			server.users = server.users[:len(server.users)-1]
		}
	}
	server.broadcastToClients(message.encode())
}

func (server *WsServer) handleUserJoinedPrivate(message Message) {
	targetClient := server.findClientByID(message.Sender.GetId())
	if targetClient != nil {
		targetClient.joinRoom(message.Target.GetName(), message.Sender)
	}
}

func (server *WsServer) findUserByID(ID string) models.User {
	var foundUser models.User

	for _, client := range server.users {
		if client.GetId() == ID {
			foundUser = client
			break
		}
	}
	return foundUser
}
