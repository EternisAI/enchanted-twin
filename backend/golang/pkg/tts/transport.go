package tts

import (
	"context"
	"net/http"

	"github.com/pion/webrtc/v4"
)

type offerMsg struct {
	SDP  string `json:"sdp"`
	Text string `json:"text"`
}

func (s *Service) handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("failed to upgrade to websocket", "error", err)
		return
	}
	defer func() {
		err := ws.Close()
		if err != nil {
			s.logger.Error("failed to close websocket", "error", err)
		}
	}()

	var msg offerMsg
	if err = ws.ReadJSON(&msg); err != nil {
		s.logger.Error("failed to read json", "error", err)
		return
	}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		s.logger.Error("failed to create peer connection", "error", err)
		return
	}
	defer func() {
		err := pc.Close()
		if err != nil {
			s.logger.Error("failed to close peer connection", "error", err)
		}
	}()

	dc, err := pc.CreateDataChannel("audio", nil)
	if err != nil {
		s.logger.Error("failed to create data channel", "error", err)
		return
	}

	err = pc.SetRemoteDescription(
		webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: msg.SDP},
	)
	if err != nil {
		s.logger.Error("failed to set remote description", "error", err)
		return
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		s.logger.Error("failed to create answer", "error", err)
		return
	}
	err = pc.SetLocalDescription(answer)
	if err != nil {
		s.logger.Error("failed to set local description", "error", err)
		return
	}
	<-webrtc.GatheringCompletePromise(pc)
	err = ws.WriteJSON(pc.LocalDescription())
	if err != nil {
		s.logger.Error("failed to write json", "error", err)
		return
	}

	dc.OnOpen(func() { go s.pipe(r.Context(), dc, msg.Text) })

	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}
}

func (s *Service) pipe(ctx context.Context, dc *webrtc.DataChannel, text string) {
	rc, err := s.provider.Stream(ctx, text)
	if err != nil {
		_ = dc.Close()
		return
	}
	defer func() {
		err := rc.Close()
		if err != nil {
			s.logger.Error("failed to close reader", "error", err)
		}
	}()

	buf := make([]byte, 4096)
	for {
		n, err := rc.Read(buf)
		if n > 0 {
			_ = dc.Send(buf[:n])
		}
		if err != nil {
			break
		}
	}

	_ = dc.SendText("EOS")
	err = dc.Close()
	if err != nil {
		s.logger.Error("failed to close data channel", "error", err)
	}
}
