package tts

import (
	"context"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

type Service struct {
	addr     string
	provider TTSProvider
	upgrader websocket.Upgrader
	logger   log.Logger
}

func New(addr string, p TTSProvider, logger log.Logger) *Service {
	return &Service{
		addr:     addr,
		provider: p,
		logger:   logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

func (s *Service) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)

	srv := &http.Server{Addr: s.addr, Handler: mux}

	// graceful shutdown
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	s.logger.Info("Started TTS service on", "host", "ws://localhost"+s.addr+"/ws")
	return srv.ListenAndServe()
}

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
	defer ws.Close()

	var msg offerMsg
	if err = ws.ReadJSON(&msg); err != nil {
		s.logger.Error("failed to read json", "error", err)
		return
	}

	pc, _ := webrtc.NewPeerConnection(webrtc.Configuration{})
	defer pc.Close()

	dc, _ := pc.CreateDataChannel("audio", nil)

	_ = pc.SetRemoteDescription(
		webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: msg.SDP},
	)
	answer, _ := pc.CreateAnswer(nil)
	_ = pc.SetLocalDescription(answer)
	<-webrtc.GatheringCompletePromise(pc)
	_ = ws.WriteJSON(pc.LocalDescription())

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
		return
	}
	defer rc.Close()

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
}
