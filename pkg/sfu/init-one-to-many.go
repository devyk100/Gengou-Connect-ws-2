package sfu

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v4"
	"net/http"
	"os"
	"ws-sfu-server/pkg/misc"
	"ws-sfu-server/pkg/types"
)

type SfuPayload struct {
	SDP     string `json:"sdp"`
	Secret  string `json:"secret"`
	ClassId string `json:"classId"`
	Success bool   `json:"success"`
}

var peerConnectionConfig *webrtc.Configuration
var interceptorRegistry *interceptor.Registry
var mediaEngine *webrtc.MediaEngine
var intervalPliFactory *intervalpli.ReceiverInterceptorFactory

func HandleInitConnection(writer http.ResponseWriter, request *http.Request) {
	err := godotenv.Load(".env")
	if err != nil {
		panic(err.Error())
	}
	turnIp := os.Getenv("TURN_IP")

	if peerConnectionConfig == nil {
		peerConnectionConfig = &webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{
					URLs: []string{
						"stun:" + turnIp + ":3478",
						"stun:stun.l.google.com:19302",
						"turn:" + turnIp + ":3478",
					},
					Username:       "user",
					Credential:     "pass",
					CredentialType: 0,
				},
			},
		}
	}

	conn, err := misc.WsConnectionUpgrader.Upgrade(writer, request, nil)
	if err != nil {
		panic(err.Error())
	}

	payload := SfuPayload{}

	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			panic(err.Error())
		}
	}(conn)

	if request.URL.Query().Get("user") == types.Instructor {
		err := conn.ReadJSON(&payload) // read the initial one for the sdp offer
		if err != nil {
			panic(err.Error())
		}

		LiveClasses[payload.ClassId].InstructorWsConnection = conn

		offer := webrtc.SessionDescription{}

		decodePayload(&payload, &offer)

		if mediaEngine == nil {
			mediaEngine = &webrtc.MediaEngine{}
			err = mediaEngine.RegisterDefaultCodecs()
			if err != nil {
				panic(err.Error())
			}
		}
		if interceptorRegistry == nil {
			interceptorRegistry = &interceptor.Registry{}
			err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry)
			if err != nil {
				panic(err.Error())
			}
		}

		if intervalPliFactory == nil {
			intervalPliFactory, err = intervalpli.NewReceiverInterceptor()
			if err != nil {
				panic(err.Error())
			}
			interceptorRegistry.Add(intervalPliFactory)
		}

		peerConnection, err := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine), webrtc.WithInterceptorRegistry(interceptorRegistry)).NewPeerConnection(*peerConnectionConfig)
		if err != nil {
			panic(err.Error())
		}

		_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo)
		if err != nil {
			panic(err.Error())
		}

		err = peerConnection.SetRemoteDescription(offer)
		if err != nil {
			panic(err.Error())
		}

		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			panic(err.Error())
		}

		gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

		err = peerConnection.SetLocalDescription(answer)
		if err != nil {
			panic(err.Error())
		}

		// blocking the thread until the gathering of the ICE candidates is complete
		<-gatherComplete

		sdpString := encodeToBase64(&answer)
		err = conn.WriteJSON(&SfuPayload{
			SDP:     sdpString,
			Secret:  "success",
			Success: true,
			ClassId: payload.ClassId,
		})
		if LiveClasses[payload.ClassId] == nil {
			LiveClasses[payload.ClassId] = &LiveClass{}
		}
		LiveClasses[payload.ClassId].InstructorPeerConnection = peerConnection
		LiveClasses[payload.ClassId].ClassId = payload.ClassId

		if LiveClasses[payload.ClassId].WaitingLearnerChannel != nil {
			for index := len(LiveClasses[payload.ClassId].WaitingLearnerChannel) - 1; index >= 0; index-- {
				LiveClasses[payload.ClassId].WaitingLearnerChannel <- true
			}
		}

		// when the instructor disconnects, the learner must repeat the websocket connection once more, and stall the thread until the instructor connects again
		peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			if state == webrtc.PeerConnectionStateDisconnected {
				for _, Conn := range LiveClasses[payload.ClassId].LearnerWsConnection {
					err := Conn.WriteJSON(&SfuPayload{
						SDP:     "",
						Secret:  "",
						ClassId: "",
						Success: false,
					})
					if err != nil {
						panic(err.Error())
					}
				}

				for _, Peer := range LiveClasses[payload.ClassId].LearnerPeerConnections {
					err := Peer.Close()
					if err != nil {
						panic(err.Error())
					}
				}

				LiveClasses[payload.ClassId].InstructorPeerConnection = nil
				LiveClasses[payload.ClassId].InstructorWsConnection = nil

				return
			}
		})

		for {
			fmt.Println("Attempting to read from the instructor")
			err := conn.ReadJSON(&payload)
			if LiveClasses[payload.ClassId].InstructorPeerConnection != nil {
				return
			}
			if err != nil {
				panic(err.Error())
				return
			}
			fmt.Println("Received a payload from the instructor", payload.SDP)
		}
	} else {
		// the learner case
		receiverOnlyOffer := webrtc.SessionDescription{}
		decodePayload(&payload, &receiverOnlyOffer)

		peerConnection, err := webrtc.NewPeerConnection(*peerConnectionConfig)
		if err != nil {
			panic(err.Error())
		}

		if LiveClasses[payload.ClassId] == nil {
			LiveClasses[payload.ClassId] = &LiveClass{}
		}

		if LiveClasses[payload.ClassId].InstructorPeerConnection == nil {
			//err := conn.WriteJSON(&SfuPayload{
			//	SDP:     "",
			//	Secret:  "",
			//	ClassId: payload.ClassId,
			//	Success: false,
			//})
			//if err != nil {
			//	panic(err.Error())
			//	return
			//}

			LiveClasses[payload.ClassId].LearnerPeerConnections = append(LiveClasses[payload.ClassId].LearnerPeerConnections, peerConnection)

			LiveClasses[payload.ClassId].WaitingLearnerChannelMutex.Lock()
			// the waiters for the instructor to connect
			if LiveClasses[payload.ClassId].WaitingLearnerChannel == nil {
				LiveClasses[payload.ClassId].WaitingLearnerChannel = make(chan bool, len(LiveClasses[payload.ClassId].LearnerPeerConnections))
			} else {
				LiveClasses[payload.ClassId].WaitingLearnerChannel = nil
				LiveClasses[payload.ClassId].WaitingLearnerChannel = make(chan bool, len(LiveClasses[payload.ClassId].LearnerPeerConnections))
			}
			LiveClasses[payload.ClassId].WaitingLearnerChannelMutex.Unlock()

			<-LiveClasses[payload.ClassId].WaitingLearnerChannel
		}

		rtpSender, err := peerConnection.AddTrack(LiveClasses[payload.ClassId].LocalVideoTrack)
		if err != nil {
			panic(err.Error())
		}

		_, err = peerConnection.AddTrack(LiveClasses[payload.ClassId].LocalAudioTrack)
		if err != nil {
			panic(err.Error())
		}

		// necessary to keep the webrtc connection alive
		go func() {
			rtpBuf := make([]byte, 1000)
			for {
				_, _, rtpErr := rtpSender.Read(rtpBuf)
				if rtpErr != nil {
					panic(rtpErr.Error())
				}
				if LiveClasses[payload.ClassId].InstructorPeerConnection == nil {
					err := conn.WriteJSON(&SfuPayload{
						SDP:     "",
						Secret:  "",
						ClassId: "",
						Success: false,
					})
					if err != nil {
						panic(err.Error())
					}
				}
			}
		}()

		err = peerConnection.SetRemoteDescription(receiverOnlyOffer)
		if err != nil {
			panic(err.Error())
		}
		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			panic(err.Error())
		}

		gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

		err = peerConnection.SetLocalDescription(answer)
		if err != nil {
			panic(err.Error())
		}

		<-gatherComplete

	}

}
