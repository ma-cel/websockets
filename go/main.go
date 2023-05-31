package main

import (
	"flag"
	"log"
	"net/http"
)

var addr = flag.String("addr", ":8080", "http server address")

func main() {
	flag.Parse()
	WsServer := NewWebsocketServer()
	go WsServer.Run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, request *http.Request) {
		ServeWs(WsServer, w, request)
	})
	log.Fatal(http.ListenAndServe(*addr, nil))
}
