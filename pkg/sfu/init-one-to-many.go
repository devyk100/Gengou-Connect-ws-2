package sfu

import (
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
	UserId     string `json:"userId"`
	SDP        string `json:"sdp"`
	Secret     string `json:"secret"`
	ClassId    string `json:"classId"`
	Disconnect bool   `json:"disconnect"`
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
						"turn:" + turnIp + ":3478",
						"stun:" + turnIp + ":3478",
						"stun:stun.l.google.com:19302",
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

	err = conn.ReadJSON(&payload)
	if err != nil {
		panic(err.Error())
	}

	if LiveClasses[payload.ClassId] == nil {
		LiveClasses[payload.ClassId] = &LiveClass{}
	}

	LiveClasses[payload.ClassId].ClassId = payload.ClassId

	if request.URL.Query().Get("user") == types.Instructor {
		err := handleInstructorOneToManyConnection(payload, conn)
		if err != nil {
			panic(err.Error())
			return
		}
	} else {
		err := handleLearnerOneToManyConnection(payload, conn)
		if err != nil {
			panic(err.Error())
			return
		}
	}

}
