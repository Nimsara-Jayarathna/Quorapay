package admin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"quorapay/internal/coordination"
)

type StatusProvider interface {
	CurrentStatus() coordination.Status
}

type Service struct {
	status         StatusProvider
	rootDir        string
	runNodeScript  string
	killNodeScript string
	zkAddr         string
}

type Result struct {
	NodeID string `json:"node_id"`
	Action string `json:"action"`
	Output string `json:"output,omitempty"`
}

func NewService(status StatusProvider, rootDir string, runNodeScript string, killNodeScript string, zkAddr string) *Service {
	return &Service{
		status:         status,
		rootDir:        strings.TrimSpace(rootDir),
		runNodeScript:  strings.TrimSpace(runNodeScript),
		killNodeScript: strings.TrimSpace(killNodeScript),
		zkAddr:         strings.TrimSpace(zkAddr),
	}
}

func (s *Service) Execute(ctx context.Context, nodeID string, action string) (Result, error) {
	id, err := normalizeNodeID(nodeID)
	if err != nil {
		return Result{}, err
	}

	action = strings.ToLower(strings.TrimSpace(action))
	if action != "start" && action != "stop" && action != "restart" {
		return Result{}, fmt.Errorf("unsupported action: %s", action)
	}

	if err := s.requireCoordinationHealthy(); err != nil {
		return Result{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	switch action {
	case "start":
		out, err := s.runCommand(ctx, s.runNodeScript, id)
		return Result{NodeID: id, Action: action, Output: out}, err
	case "stop":
		out, err := s.runCommand(ctx, s.killNodeScript, id)
		return Result{NodeID: id, Action: action, Output: out}, err
	default:
		stopOut, stopErr := s.runCommand(ctx, s.killNodeScript, id)
		startOut, startErr := s.runCommand(ctx, s.runNodeScript, id)
		combined := strings.TrimSpace(strings.TrimSpace(stopOut) + "\n" + strings.TrimSpace(startOut))
		if stopErr != nil && startErr != nil {
			return Result{NodeID: id, Action: action, Output: combined}, fmt.Errorf("restart failed: stop=%v start=%v", stopErr, startErr)
		}
		if startErr != nil {
			return Result{NodeID: id, Action: action, Output: combined}, fmt.Errorf("restart failed: %w", startErr)
		}
		return Result{NodeID: id, Action: action, Output: combined}, nil
	}
}

func normalizeNodeID(nodeID string) (string, error) {
	id := strings.ToUpper(strings.TrimSpace(nodeID))
	if len(id) != 1 || id[0] < 'A' || id[0] > 'Z' {
		return "", fmt.Errorf("invalid node id: %s", nodeID)
	}
	return id, nil
}

func (s *Service) requireCoordinationHealthy() error {
	if s.status != nil {
		st := s.status.CurrentStatus()
		if strings.TrimSpace(st.ZKError) != "" {
			return fmt.Errorf("zookeeper is not healthy: %s", st.ZKError)
		}
	}
	if s.zkAddr != "" {
		conn, err := net.DialTimeout("tcp", s.zkAddr, 2*time.Second)
		if err != nil {
			return fmt.Errorf("zookeeper is not reachable at %s: %w", s.zkAddr, err)
		}
		_ = conn.Close()
	}
	return nil
}

func (s *Service) runCommand(ctx context.Context, script string, nodeID string) (string, error) {
	if strings.TrimSpace(script) == "" {
		return "", errors.New("script path is not configured")
	}
	cmd := exec.CommandContext(ctx, script, nodeID)
	if s.rootDir != "" {
		cmd.Dir = s.rootDir
	}
	output, err := cmd.CombinedOutput()
	out := strings.TrimSpace(string(output))
	if err != nil {
		if out == "" {
			return "", err
		}
		return out, fmt.Errorf("%w: %s", err, out)
	}
	return out, nil
}
