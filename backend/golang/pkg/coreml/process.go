package coreml

import (
	"bufio"
	"context"
	"io"
	"os/exec"
)

type BinaryProcess interface {
	Start(ctx context.Context, binaryPath, modelPath string) error
	Stop() error
	Write(data []byte) error
	ReadLine() (string, error)
	IsRunning() bool
	Restart(ctx context.Context, binaryPath, modelPath string) error
}

type RealBinaryProcess struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	scanner *bufio.Scanner
}

func NewRealBinaryProcess() *RealBinaryProcess {
	return &RealBinaryProcess{}
}

func (p *RealBinaryProcess) Start(ctx context.Context, binaryPath, modelPath string) error {
	p.cmd = exec.CommandContext(ctx, binaryPath, "interactive", modelPath)

	stdin, err := p.cmd.StdinPipe()
	if err != nil {
		return err
	}
	p.stdin = stdin

	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	p.stdout = stdout
	p.scanner = bufio.NewScanner(stdout)

	return p.cmd.Start()
}

func (p *RealBinaryProcess) Stop() error {
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	if p.stdout != nil {
		_ = p.stdout.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

func (p *RealBinaryProcess) Write(data []byte) error {
	if p.stdin == nil {
		return io.ErrClosedPipe
	}
	_, err := p.stdin.Write(data)
	return err
}

func (p *RealBinaryProcess) ReadLine() (string, error) {
	if p.scanner == nil {
		return "", io.ErrClosedPipe
	}
	if !p.scanner.Scan() {
		return "", p.scanner.Err()
	}
	return p.scanner.Text(), nil
}

func (p *RealBinaryProcess) IsRunning() bool {
	return p.cmd != nil && p.cmd.ProcessState == nil
}

func (p *RealBinaryProcess) Restart(ctx context.Context, binaryPath, modelPath string) error {
	_ = p.Stop()
	return p.Start(ctx, binaryPath, modelPath)
}

type MockBinaryProcess struct {
	running   bool
	responses []string
	index     int
	writeData [][]byte
}

func NewMockBinaryProcess(responses []string) *MockBinaryProcess {
	return &MockBinaryProcess{
		responses: responses,
	}
}

func (p *MockBinaryProcess) Start(ctx context.Context, binaryPath, modelPath string) error {
	p.running = true
	p.index = 0
	return nil
}

func (p *MockBinaryProcess) Stop() error {
	p.running = false
	return nil
}

func (p *MockBinaryProcess) Write(data []byte) error {
	if !p.running {
		return io.ErrClosedPipe
	}
	p.writeData = append(p.writeData, data)
	return nil
}

func (p *MockBinaryProcess) ReadLine() (string, error) {
	if !p.running {
		return "", io.ErrClosedPipe
	}
	if p.index >= len(p.responses) {
		return "", io.EOF
	}
	response := p.responses[p.index]
	p.index++
	return response, nil
}

func (p *MockBinaryProcess) IsRunning() bool {
	return p.running
}

func (p *MockBinaryProcess) Restart(ctx context.Context, binaryPath, modelPath string) error {
	return p.Start(ctx, binaryPath, modelPath)
}

func (p *MockBinaryProcess) GetWrittenData() [][]byte {
	return p.writeData
}
