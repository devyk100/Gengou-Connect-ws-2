package sfu

import (
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"sync"
)

type LiveClass struct {
	InstructorPeerConnection *webrtc.PeerConnection
	LearnerPeerConnections   map[string]*webrtc.PeerConnection
	ClassId                  string
	LocalVideoTrack          *webrtc.TrackLocalStaticRTP
	LocalAudioTrack          *webrtc.TrackLocalStaticRTP
	WaitingLearnerGroup      *sync.WaitGroup
	WaitingLearnerGroupMutex sync.Mutex
	LearnerWsConnection      map[string]*websocket.Conn
	InstructorWsConnection   *websocket.Conn
}

var LiveClasses = make(map[string]*LiveClass)
