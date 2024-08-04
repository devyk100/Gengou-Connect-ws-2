package sfu

import (
	"encoding/base64"
	"encoding/json"
	"github.com/pion/webrtc/v4"
)

func decodePayload(payload *SfuPayload, obj *webrtc.SessionDescription) {
	sdpString, err := base64.StdEncoding.DecodeString(payload.SDP)
	if err != nil {
		panic(err.Error())
	}
	err = json.Unmarshal([]byte(sdpString), obj)
	if err != nil {
		panic(err.Error())
	}
	return
}
