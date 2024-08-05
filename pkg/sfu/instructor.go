package sfu

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v4"
)

func SignalInstructorConnected(ClassId string) {
	if LiveClasses[ClassId].LocalAudioTrack == nil || LiveClasses[ClassId].LocalVideoTrack == nil {
		return
	}
	if LiveClasses[ClassId].WaitingLearnerGroup != nil {
		fmt.Println("Trying to open the thread")
		fmt.Println("the length of the waiting learner channel is", len(LiveClasses[ClassId].LearnerPeerConnections))
		LiveClasses[ClassId].WaitingLearnerGroupMutex.Lock()
		for _, _ = range LiveClasses[ClassId].LearnerPeerConnections {

			LiveClasses[ClassId].WaitingLearnerGroup.Done()
		}
		LiveClasses[ClassId].WaitingLearnerGroupMutex.Unlock()
		LiveClasses[ClassId].WaitingLearnerGroup = nil
	}

}

func handleInstructorOneToManyConnection(payload SfuPayload, conn *websocket.Conn) error {

	var err error
	fmt.Println("Reached here in the instructor")
	if err != nil {
		return err
	}
	fmt.Println(payload.SDP)
	LiveClasses[payload.ClassId].InstructorWsConnection = conn

	offer := webrtc.SessionDescription{}

	decodePayload(&payload, &offer)

	if mediaEngine == nil {
		mediaEngine = &webrtc.MediaEngine{}
		err = mediaEngine.RegisterDefaultCodecs()
		if err != nil {
			return err
		}
	}

	if interceptorRegistry == nil {
		interceptorRegistry = &interceptor.Registry{}
		err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry)
		if err != nil {
			return err
		}
	}

	if intervalPliFactory == nil {
		intervalPliFactory, err = intervalpli.NewReceiverInterceptor()
		if err != nil {
			return err
		}
		interceptorRegistry.Add(intervalPliFactory)
	}

	peerConnection, err := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithInterceptorRegistry(interceptorRegistry)).NewPeerConnection(*peerConnectionConfig)
	if err != nil {
		return err
	}

	LiveClasses[payload.ClassId].InstructorPeerConnection = peerConnection

	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo)
	if err != nil {
		return err
	}

	err = peerConnection.SetRemoteDescription(offer)
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

	// blocking the thread until the gathering of the ICE candidates is complete
	<-gatherComplete

	sdpString := encodeToBase64(&answer)
	err = conn.WriteJSON(&SfuPayload{
		SDP:        sdpString,
		Secret:     "success",
		UserId:     "server",
		Disconnect: false,
		ClassId:    payload.ClassId,
	})

	conn.SetCloseHandler(func(code int, text string) error {

		fmt.Println("Writing the JSON for instructor disconnect", "The total learners are", len(LiveClasses[payload.ClassId].LearnerWsConnection))

		for _, Conn := range LiveClasses[payload.ClassId].LearnerWsConnection {
			err := Conn.WriteJSON(&SfuPayload{
				SDP:        "",
				Secret:     "",
				ClassId:    "",
				Disconnect: true,
				UserId:     "server",
			})
			if err != nil {
				panic(err.Error())
			}
		}

		err := LiveClasses[payload.ClassId].InstructorPeerConnection.Close()
		if err != nil {
			panic(err.Error())
			return err
		}

		for _, Peer := range LiveClasses[payload.ClassId].LearnerPeerConnections {
			err := Peer.Close()
			if err != nil {
				panic(err.Error())
				return err
			}
		}

		LiveClasses[payload.ClassId].InstructorPeerConnection = nil
		LiveClasses[payload.ClassId].InstructorWsConnection = nil
		LiveClasses[payload.ClassId].LocalAudioTrack = nil
		LiveClasses[payload.ClassId].LocalVideoTrack = nil
		return nil
	})
	// when the instructor disconnects, the learner must repeat the websocket connection once more, and stall the thread until the instructor connects again

	LiveClasses[payload.ClassId].HandleBroadcast()

	for {
		fmt.Println("Attempting to read from the instructor")
		err := conn.ReadJSON(&payload)
		if LiveClasses[payload.ClassId].InstructorPeerConnection == nil {
			fmt.Println("Closed the connection")
			break
		}
		if err != nil {
			panic(err.Error())
			return err
		}
		fmt.Println("Received a payload from the instructor", payload.SDP)
	}

	return nil
}
