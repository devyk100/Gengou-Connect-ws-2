package sfu

import (
	"encoding/base64"
	"encoding/json"
	"github.com/pion/webrtc/v4"
)

func encodeToBase64(answer *webrtc.SessionDescription) string {
	jsonSdpString, err := json.Marshal(answer)
	if err != nil {
		panic(err.Error())
	}
	return base64.StdEncoding.EncodeToString(jsonSdpString)
}
