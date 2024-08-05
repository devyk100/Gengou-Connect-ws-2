package sfu

import (
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"sync"
)

type LiveClass struct {
	ClassId                  string
	InstructorPeerConnection *webrtc.PeerConnection
	InstructorWsConnection   *websocket.Conn
	LocalVideoTrack          *webrtc.TrackLocalStaticRTP
	LocalAudioTrack          *webrtc.TrackLocalStaticRTP
	LearnerWsConnection      map[string]*websocket.Conn
	LearnerPeerConnections   map[string]*webrtc.PeerConnection
	WaitingLearnerGroup      *sync.WaitGroup
	WaitingLearnerBroadcast  *sync.Cond
	WaitingLearnerGroupMutex sync.Mutex
}

var LiveClasses = make(map[string]*LiveClass)
