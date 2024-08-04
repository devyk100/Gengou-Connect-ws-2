package sfu

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v4"
	"io"
	"net/http"
	"sync"
	"ws-sfu-server/pkg/misc"
	"ws-sfu-server/pkg/types"
)

type SfuPayload struct {
	SDP                   string `json:"sdp"`
	Secret                string `json:"secret"`
	ClassId               string `json:"classId"`
	Success               bool   `json:"success"`
	IsInstructorConnected bool   `json:"isInstructorConnected"`
	UserId                string `json:"userId"`
}

var peerConnectionConfig *webrtc.Configuration
var interceptorRegistry *interceptor.Registry
var mediaEngine *webrtc.MediaEngine
var intervalPliFactory *intervalpli.ReceiverInterceptorFactory

func SignalInstructorConnected(ClassId string) {
	if LiveClasses[ClassId].LocalAudioTrack == nil || LiveClasses[ClassId].LocalVideoTrack == nil {
		return
	}
	if LiveClasses[ClassId].WaitingLearnerGroup != nil {
		fmt.Println("Trying to open the thread")
		fmt.Println("the length of the waiting learner channel is", len(LiveClasses[ClassId].LearnerPeerConnections))

		for index := 0; index < len(LiveClasses[ClassId].LearnerPeerConnections); index++ {
			LiveClasses[ClassId].WaitingLearnerGroup.Done()
		}
		LiveClasses[ClassId].WaitingLearnerGroup = nil
	}
}

func HandleInitConnection(writer http.ResponseWriter, request *http.Request) {
	err := godotenv.Load(".env")
	if err != nil {
		panic(err.Error())
	}
	//turnIp := os.Getenv("TURN_IP")

	if peerConnectionConfig == nil {
		peerConnectionConfig = &webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{
					URLs: []string{
						//"stun:" + turnIp + ":3478",
						"stun:stun.l.google.com:19302",
						//"turn:" + turnIp + ":3478",
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

	err = conn.ReadJSON(&payload)
	if err != nil {
		panic(err.Error())
	}

	if LiveClasses[payload.ClassId] == nil {
		LiveClasses[payload.ClassId] = &LiveClass{}
	}
	err = conn.WriteJSON(&SfuPayload{
		SDP:                   "",
		Secret:                "",
		ClassId:               "",
		Success:               true,
		IsInstructorConnected: LiveClasses[payload.ClassId].InstructorPeerConnection != nil,
		// read the initial one for the sdp offer
	})
	if err != nil {
		panic(err.Error())
	}

	if request.URL.Query().Get("user") == types.Instructor {
		fmt.Println("Reached here")
		if err != nil {
			panic(err.Error())
		}
		fmt.Println(payload.SDP)
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
		fmt.Println("It works 180")

		conn.SetCloseHandler(func(code int, text string) error {
			fmt.Println("Writing the JSON for instructor disconnect")
			for _, Conn := range LiveClasses[payload.ClassId].LearnerWsConnection {
				err := Conn.WriteJSON(&SfuPayload{
					SDP:                   "",
					Secret:                "",
					ClassId:               "",
					Success:               false,
					IsInstructorConnected: false,
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
			return nil
		})
		// when the instructor disconnects, the learner must repeat the websocket connection once more, and stall the thread until the instructor connects again

		LiveClasses[payload.ClassId].HandleBroadcast()

		for {
			fmt.Println("Attempting to read from the instructor")
			err := conn.ReadJSON(&payload)
			if LiveClasses[payload.ClassId].InstructorPeerConnection == nil {
				return
			}
			if err != nil {
				if websocket.IsCloseError(err) || err == io.EOF {
					panic("Connection closed:" + err.Error())
				}
				panic(err.Error())
			}
			fmt.Println("Received a payload from the instructor", payload.SDP)
		}
	} else {
		// the learner case
		fmt.Println(payload.SDP)
		receiverOnlyOffer := webrtc.SessionDescription{}
		decodePayload(&payload, &receiverOnlyOffer)

		peerConnection, err := webrtc.NewPeerConnection(*peerConnectionConfig)
		if err != nil {
			panic(err.Error())
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
			delete(LiveClasses[payload.ClassId].LearnerWsConnection, payload.UserId)
			delete(LiveClasses[payload.ClassId].LearnerPeerConnections, payload.UserId)
			return nil
		})

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

			fmt.Println("the stalled thread")

			LiveClasses[payload.ClassId].WaitingLearnerGroupMutex.Lock()

			// the waiters for the instructor to connect
			if LiveClasses[payload.ClassId].WaitingLearnerGroup == nil {
				LiveClasses[payload.ClassId].WaitingLearnerGroup = &sync.WaitGroup{}
			}
			LiveClasses[payload.ClassId].WaitingLearnerGroup.Add(1)

			LiveClasses[payload.ClassId].WaitingLearnerGroupMutex.Unlock()
			fmt.Println("just before the stalling of thread")
			LiveClasses[payload.ClassId].WaitingLearnerGroup.Wait()
			fmt.Println("Just after that")
		}
		fmt.Println(LiveClasses[payload.ClassId].LocalVideoTrack)
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
				if LiveClasses[payload.ClassId].InstructorPeerConnection == nil {
					err := conn.WriteJSON(&SfuPayload{
						SDP:     "",
						Secret:  "",
						ClassId: "",
						Success: false,
					})
					if err != nil {
						panic(err.Error())
						return
					}
				}
				if LiveClasses[payload.ClassId].LearnerPeerConnections == nil {
					fmt.Println("Was deleted")
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

		sdpString := encodeToBase64(&answer)
		err = conn.WriteJSON(&SfuPayload{
			SDP:     sdpString,
			Secret:  "secret",
			ClassId: payload.ClassId,
			Success: true,
		})
		if err != nil {
			panic(err.Error())
		}

		for {
			fmt.Println("Attempting to read from the learner")
			err := conn.ReadJSON(&payload)
			if LiveClasses[payload.ClassId].InstructorPeerConnection == nil {
				return
			}
			if LiveClasses[payload.ClassId].LearnerPeerConnections[payload.UserId] == nil {
				return
			}
			if err != nil {
				if websocket.IsCloseError(err) || err == io.EOF {
					panic("Connection closed:" + err.Error())
				}
				panic(err.Error())
			}
			fmt.Println("Received a payload from the instructor", payload.SDP)
		}
	}

	return

}
