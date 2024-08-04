package misc

import (
	"github.com/gorilla/websocket"
	"net/http"
)

var WsConnectionUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
