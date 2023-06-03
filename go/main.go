package main

import (
	"flag"
	"log"
	"net/http"
	"websockets/config"
	"websockets/repositories"
)

var addr = flag.String("addr", ":8080", "http server address")

func main() {
	flag.Parse()
	WsServer := NewWebsocketServer(&repositories.RoomRepository{Db: db}, &repositories.UserRepository{Db: db})
	go WsServer.Run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, request *http.Request) {
		ServeWs(WsServer, w, request)
	})
	db := config.InitDB()
	defer db.Close()

	log.Fatal(http.ListenAndServe(*addr, nil))
}
