package sfu

import (
	"fmt"
	"github.com/pion/webrtc/v4"
	"io"
)

func (liveClass *LiveClass) HandleBroadcast() {
	liveClass.InstructorPeerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		trackType := remoteTrack.Kind().String()
		if trackType == "video" {
			localTrack, newTrackErr := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, "video", liveClass.ClassId)
			if newTrackErr != nil {
				panic(newTrackErr.Error())
			}
			liveClass.LocalVideoTrack = localTrack

			rtpBuffer := make([]byte, 5000)
			SignalInstructorConnected(liveClass.ClassId)
			for {
				i, _, readErr := remoteTrack.Read(rtpBuffer)
				if readErr != nil {
					if readErr == io.EOF {
						return
					}
					panic(readErr.Error())
				}

				_, err := localTrack.Write(rtpBuffer[:i])
				if err != nil {
					fmt.Println(err.Error())
				}
			}
		} else if trackType == "audio" {
			fmt.Println("Audio track testing")
			localTrack, newTrackErr := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, "audio", liveClass.ClassId)
			if newTrackErr != nil {
				panic(newTrackErr.Error())
			}

			liveClass.LocalAudioTrack = localTrack

			rtpBuffer := make([]byte, 5000)
			SignalInstructorConnected(liveClass.ClassId)
			for {
				i, _, readErr := remoteTrack.Read(rtpBuffer)
				if readErr != nil {
					if readErr == io.EOF {
						return
					}
					panic(readErr.Error())
				}

				_, err := localTrack.Write(rtpBuffer[:i])
				if err != nil {
					fmt.Println(err.Error())
					return
				}
			}
		}
	})
}
