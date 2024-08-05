package sfu

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"io"
	"sync"
)

func handleLearnerOneToManyConnection(payload SfuPayload, conn *websocket.Conn) error {
	fmt.Println(payload.SDP)
	var err error

	receiverOnlyOffer := webrtc.SessionDescription{}
	decodePayload(&payload, &receiverOnlyOffer)

	peerConnection, err := webrtc.NewPeerConnection(*peerConnectionConfig)
	if err != nil {
		return err
	}

	if LiveClasses[payload.ClassId] == nil {
		LiveClasses[payload.ClassId] = &LiveClass{}
	}

	if LiveClasses[payload.ClassId].LearnerWsConnection == nil {
		LiveClasses[payload.ClassId].LearnerWsConnection = make(map[string]*websocket.Conn)
		LiveClasses[payload.ClassId].LearnerPeerConnections = make(map[string]*webrtc.PeerConnection)
	}

	LiveClasses[payload.ClassId].LearnerWsConnection[payload.UserId] = conn
	LiveClasses[payload.ClassId].LearnerPeerConnections[payload.UserId] = peerConnection

	conn.SetCloseHandler(func(code int, text string) error {
		//LiveClasses[payload.ClassId].WaitingLearnerGroup.Done()
		fmt.Println("The learner is trying to disconnect.")
		delete(LiveClasses[payload.ClassId].LearnerWsConnection, payload.UserId)
		delete(LiveClasses[payload.ClassId].LearnerPeerConnections, payload.UserId)
		return nil
	})

	if LiveClasses[payload.ClassId].InstructorPeerConnection == nil {
		fmt.Println("Stalling the thread")

		LiveClasses[payload.ClassId].WaitingLearnerGroupMutex.Lock()
		// the waiters for the instructor to connect
		if LiveClasses[payload.ClassId].WaitingLearnerGroup == nil {
			LiveClasses[payload.ClassId].WaitingLearnerGroup = &sync.WaitGroup{}
		}
		LiveClasses[payload.ClassId].WaitingLearnerGroup.Add(1)

		LiveClasses[payload.ClassId].WaitingLearnerGroupMutex.Unlock()

		LiveClasses[payload.ClassId].WaitingLearnerGroup.Wait()
		fmt.Println("Just after stall thread opens")
	}
	fmt.Println(LiveClasses[payload.ClassId].LocalVideoTrack, "Got the tracks")
	rtpSender, err := peerConnection.AddTrack(LiveClasses[payload.ClassId].LocalVideoTrack)
	if err != nil {
		return err
	}

	fmt.Println(LiveClasses[payload.ClassId].LocalAudioTrack, "Got the audio track")
	_, err = peerConnection.AddTrack(LiveClasses[payload.ClassId].LocalAudioTrack)
	if err != nil {
		return err
	}

	// necessary to keep the webrtc connection alive
	go func() {
		rtpBuf := make([]byte, 1000)
		for {
			_, _, rtpErr := rtpSender.Read(rtpBuf)
			if LiveClasses[payload.ClassId].InstructorWsConnection == nil {
				fmt.Println("We guess the instructor was disconnected")
				return
			}
			if rtpErr != nil {
				if rtpErr == io.EOF {
					return
				}
				panic(rtpErr.Error())
			}
		}
	}()

	err = peerConnection.SetRemoteDescription(receiverOnlyOffer)
	if err != nil {
		return err
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return err
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		return err
	}

	<-gatherComplete

	sdpString := encodeToBase64(&answer)
	err = conn.WriteJSON(&SfuPayload{
		SDP:        sdpString,
		Secret:     "secret",
		ClassId:    payload.ClassId,
		UserId:     "server",
		Disconnect: false,
	})
	if err != nil {
		return err
	}

	for {
		fmt.Println("Attempting to read from the learner")
		err := conn.ReadJSON(&payload)
		if LiveClasses[payload.ClassId].InstructorPeerConnection == nil {
			fmt.Println("We guess the instructor was disconnected")
			return err
		}
		if LiveClasses[payload.ClassId].LearnerPeerConnections[payload.UserId] == nil {
			fmt.Println("We guess the learner itself was disconnected")
			return err
		}
		if err != nil {
			if websocket.IsCloseError(err) || err == io.EOF {
				panic("Connection closed:" + err.Error())
			}
			return err
		}
		fmt.Println("Received a payload from the instructor", payload.SDP)
	}
	return nil
}
