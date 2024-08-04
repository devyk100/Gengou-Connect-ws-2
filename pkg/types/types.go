package types

import "github.com/gorilla/websocket"

const (
	Instructor = "0"
	Learner    = "1"
)

type Client struct {
	UserId     string
	ClassId    string
	Conn       *websocket.Conn
	IsVerified bool
	Type       string
}

type Event struct {
	Type string
	Key  string
}
