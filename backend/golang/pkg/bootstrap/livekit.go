// Owner: august@eternis.ai
package bootstrap

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/charmbracelet/log"
	lkauth "github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pkg/errors"
)

// LiveKitServer manages the LiveKit server process and provides client functionality
type LiveKitServer struct {
	logger     *log.Logger
	process    *exec.Cmd
	url        string
	apiKey     string
	apiSecret  string
	port       string
	roomAPI    *lksdk.RoomServiceClient
	configPath string
}

// Room represents a LiveKit room
type Room struct {
	Name string
	SID  string
}

// StartLiveKitServer starts the LiveKit server process and returns a management instance
func StartLiveKitServer(logger *log.Logger, port, apiKey, apiSecret string) (*LiveKitServer, error) {
	server := &LiveKitServer{
		logger:    logger,
		url:       fmt.Sprintf("ws://localhost:%s", port),
		apiKey:    apiKey,
		apiSecret: apiSecret,
		port:      port,
	}

	// Ensure LiveKit binary is available
	binaryPath, err := server.ensureLiveKitBinary()
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure LiveKit binary")
	}

	// Create config file
	if err := server.createConfigFile(); err != nil {
		return nil, errors.Wrap(err, "failed to create config file")
	}

	// Start the server process
	if err := server.startProcess(binaryPath); err != nil {
		return nil, errors.Wrap(err, "failed to start LiveKit server process")
	}

	// Wait for server to be ready
	if err := server.waitForReady(); err != nil {
		server.Stop()
		return nil, errors.Wrap(err, "LiveKit server failed to start")
	}

	// Initialize room API client
	server.roomAPI = lksdk.NewRoomServiceClient(server.url, server.apiKey, server.apiSecret)

	logger.Info("LiveKit server started successfully", "port", port, "url", server.url)
	return server, nil
}

// ensureLiveKitBinary downloads or finds the LiveKit server binary
func (s *LiveKitServer) ensureLiveKitBinary() (string, error) {
	// First check if livekit-server is in PATH
	if path, err := exec.LookPath("livekit-server"); err == nil {
		s.logger.Info("Found LiveKit server in PATH", "path", path)
		return path, nil
	}

	// Try to download binary to local directory
	return s.downloadLiveKitBinary()
}

// downloadLiveKitBinary downloads the LiveKit server binary
func (s *LiveKitServer) downloadLiveKitBinary() (string, error) {
	version := "v1.5.2" // Latest stable version
	var filename string
	var url string

	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			filename = "livekit_Darwin_arm64.tar.gz"
		} else {
			filename = "livekit_Darwin_x86_64.tar.gz"
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			filename = "livekit_Linux_arm64.tar.gz"
		} else {
			filename = "livekit_Linux_x86_64.tar.gz"
		}
	case "windows":
		filename = "livekit_Windows_x86_64.tar.gz"
	default:
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	url = fmt.Sprintf("https://github.com/livekit/livekit/releases/download/%s/%s", version, filename)

	binDir := filepath.Join(os.TempDir(), "livekit")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create bin directory")
	}

	binaryPath := filepath.Join(binDir, "livekit-server")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// Check if binary already exists
	if _, err := os.Stat(binaryPath); err == nil {
		s.logger.Info("Using existing LiveKit binary", "path", binaryPath)
		return binaryPath, nil
	}

	s.logger.Info("Downloading LiveKit server binary", "url", url)

	// Download the archive
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Wrap(err, "failed to download LiveKit binary")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to download LiveKit binary: HTTP %d", resp.StatusCode)
	}

	// For simplicity, just extract the binary (assuming tar.gz contains livekit-server binary)
	// In a production implementation, you'd properly extract the tar.gz file
	archivePath := filepath.Join(binDir, filename)
	file, err := os.Create(archivePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to create archive file")
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to write archive file")
	}

	// For now, just use a simple approach - try to run the downloaded binary directly
	// In production, you'd extract the tar.gz properly
	s.logger.Warn("Binary download attempted but extraction not implemented. Please install livekit-server manually.")
	return "", fmt.Errorf("please install livekit-server binary manually: 'brew install livekit' on macOS")
}

// createConfigFile creates a temporary config file for the LiveKit server
func (s *LiveKitServer) createConfigFile() error {
	configContent := fmt.Sprintf(`port: %s
log_level: info
rtc:
  tcp_port: %d
  port_range_start: 50000
  port_range_end: 50100
  use_external_ip: false
keys:
  %s: %s
`, s.port, 7881, s.apiKey, s.apiSecret)

	configFile, err := os.CreateTemp("", "livekit-*.yaml")
	if err != nil {
		return errors.Wrap(err, "failed to create config file")
	}
	defer configFile.Close()

	if _, err := configFile.WriteString(configContent); err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	s.configPath = configFile.Name()
	s.logger.Info("Created LiveKit config file", "path", s.configPath)
	return nil
}

// startProcess starts the LiveKit server process
func (s *LiveKitServer) startProcess(binaryPath string) error {
	args := []string{"--config", s.configPath}

	s.process = exec.Command(binaryPath, args...)

	// Log output for debugging
	s.process.Stdout = &logWriter{logger: s.logger, prefix: "livekit-stdout"}
	s.process.Stderr = &logWriter{logger: s.logger, prefix: "livekit-stderr"}

	if err := s.process.Start(); err != nil {
		return errors.Wrap(err, "failed to start LiveKit process")
	}

	s.logger.Info("LiveKit server process started", "pid", s.process.Process.Pid)
	return nil
}

// waitForReady waits for the LiveKit server to be ready
func (s *LiveKitServer) waitForReady() error {
	// Try to connect to the port to see if the server is accepting connections
	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%s", s.port), 1*time.Second)
		if err == nil {
			conn.Close()
			s.logger.Info("LiveKit server is accepting connections")
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("LiveKit server did not become ready within 30 seconds")
}

// Stop stops the LiveKit server process
func (s *LiveKitServer) Stop() error {
	if s.process != nil {
		s.logger.Info("Stopping LiveKit server process")

		// Try graceful shutdown first
		if err := s.process.Process.Signal(os.Interrupt); err != nil {
			s.logger.Warn("Failed to send interrupt signal, force killing", "error", err)
			s.process.Process.Kill()
		} else {
			// Wait for graceful shutdown
			done := make(chan error, 1)
			go func() {
				done <- s.process.Wait()
			}()

			select {
			case err := <-done:
				if err != nil {
					s.logger.Info("LiveKit server stopped with error", "error", err)
				} else {
					s.logger.Info("LiveKit server stopped gracefully")
				}
			case <-time.After(5 * time.Second):
				s.logger.Warn("LiveKit server did not stop gracefully, force killing")
				s.process.Process.Kill()
				<-done
			}
		}
	}

	// Clean up config file
	if s.configPath != "" {
		os.Remove(s.configPath)
	}

	return nil
}

// CreateRoom creates a new LiveKit room
func (s *LiveKitServer) CreateRoom(roomName string) (*Room, error) {
	req := &livekit.CreateRoomRequest{
		Name:            roomName,
		MaxParticipants: 2,   // Only 1-1 conversations
		EmptyTimeout:    300, // 5 minutes in seconds
		Metadata:        "voice-assistant-room",
	}

	room, err := s.roomAPI.CreateRoom(context.Background(), req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create LiveKit room")
	}

	s.logger.Info("Created LiveKit room", "name", roomName, "room_id", room.Name)
	return &Room{
		Name: room.Name,
		SID:  room.Sid,
	}, nil
}

// GenerateAccessToken generates an access token for joining a room
func (s *LiveKitServer) GenerateAccessToken(roomName, participantName string) (string, error) {
	at := lkauth.NewAccessToken(s.apiKey, s.apiSecret)
	grant := &lkauth.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
	}
	at.AddGrant(grant).SetIdentity(participantName).SetValidFor(time.Hour)

	accessToken, err := at.ToJWT()
	if err != nil {
		return "", errors.Wrap(err, "failed to generate access token")
	}

	s.logger.Info("Generated access token", "room", roomName, "participant", participantName)
	return accessToken, nil
}

// logWriter implements io.Writer to redirect process output to our logger
type logWriter struct {
	logger *log.Logger
	prefix string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logger.Debug(w.prefix, "output", string(p))
	return len(p), nil
}
