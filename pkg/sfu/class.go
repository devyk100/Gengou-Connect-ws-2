package sfu

import (
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"sync"
)

type LiveClass struct {
	InstructorPeerConnection   *webrtc.PeerConnection
	LearnerPeerConnections     []*webrtc.PeerConnection
	ClassId                    string
	LocalVideoTrack            *webrtc.TrackLocalStaticRTP
	LocalAudioTrack            *webrtc.TrackLocalStaticRTP
	WaitingLearnerChannel      chan bool
	WaitingLearnerChannelMutex sync.Mutex
	LearnerWsConnection        []*websocket.Conn
	InstructorWsConnection     *websocket.Conn
}

var LiveClasses = make(map[string]*LiveClass)
