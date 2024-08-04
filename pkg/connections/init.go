package connections

import (
	"github.com/gorilla/websocket"
	"net/http"
	"ws-sfu-server/pkg/misc"
	"ws-sfu-server/pkg/types"
)

func HandleInitConnection(writer http.ResponseWriter, request *http.Request) {

	/*
		Upgrading the http to websocket connection
	*/
	conn, err := misc.WsConnectionUpgrader.Upgrade(writer, request, nil)
	if err != nil {
		panic("error at upgrade: " + err.Error())
	}

	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			panic(err.Error() + " at defer of Handle Init connection method")
		}
	}(conn)

	client := &types.Client{
		UserId:     "",
		ClassId:    "",
		Conn:       conn,
		IsVerified: false,
	}

	if request.URL.Query().Get("type") == types.Instructor {
		client.Type = types.Instructor
		err := instructorConnectionHandler(client)
		if err != nil {
			panic(err.Error() + " at Handle Init connection method inside instructor block")
		}
		return
	} else if request.URL.Query().Get("type") == types.Learner {
		client.Type = types.Learner
		err := learnerConnectionHandler(client)
		if err != nil {
			panic(err.Error() + " at Handle Init connection method inside learner block")
		}
		return
	} else {
		// miscellaneous stuff in here
	}

}
