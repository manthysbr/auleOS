package watchdog

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// Config holds the server configuration
type Config struct {
	Port       int
	SocketPath string // If set, listen on this Unix socket instead of TCP
}

// Server is the HTTP server for the watchdog
type Server struct {
	server *http.Server
	logger *slog.Logger
	cfg    Config
}

// Ensure Server implements StrictServerInterface
var _ StrictServerInterface = (*Server)(nil)

// NewServer creates a new watchdog server
func NewServer(cfg Config) *Server {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	
	s := &Server{
		logger: logger,
		cfg:    cfg,
	}

	// Create the strict handler
	handler := NewStrictHandler(s, nil)

	// Mount on mux
	mux := http.NewServeMux()
	HandlerFromMux(handler, mux)

	s.server = &http.Server{
		Handler: mux,
	}
	
	if cfg.SocketPath == "" {
		s.server.Addr = fmt.Sprintf(":%d", cfg.Port)
	}

	return s
}

// Start runs the server
func (s *Server) Start() error {
	s.logger.Info("starting watchdog server")
	
	if s.cfg.SocketPath != "" {
		// Clean up old socket
		_ = os.Remove(s.cfg.SocketPath)
		
		listener, err := net.Listen("unix", s.cfg.SocketPath)
		if err != nil {
			return fmt.Errorf("failed to listen on socket %s: %w", s.cfg.SocketPath, err)
		}
		
		// Ensure permissions (anyone can write, so Kernel can access from host if mapped properly)
		// Inside container, root/aule user matters.
		if err := os.Chmod(s.cfg.SocketPath, 0777); err != nil {
			s.logger.Warn("failed to chmod socket", "error", err)
		}
		
		s.logger.Info("listening on unix socket", "path", s.cfg.SocketPath)
		if err := s.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("watchdog server error: %w", err)
		}
		return nil
	}
	
	s.logger.Info("listening on tcp", "addr", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("watchdog server error: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// GetHealth implements StrictServerInterface
func (s *Server) GetHealth(ctx context.Context, request GetHealthRequestObject) (GetHealthResponseObject, error) {
	return GetHealth200JSONResponse{
		Status: Alive,
	}, nil
}

// ExecuteCommand implements StrictServerInterface
func (s *Server) ExecuteCommand(ctx context.Context, request ExecuteCommandRequestObject) (ExecuteCommandResponseObject, error) {
	req := request.Body
	
	if req.Command == "" {
		msg := "command is required"
		return ExecuteCommand500JSONResponse{Error: msg}, nil // Should probably be 400, but spec says 500 for error for now or I need to update spec to allow 400
	}

	// Prepare the command
	cmdCtx := ctx
	if req.TimeoutMs != nil && *req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, time.Duration(*req.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	var args []string
	if req.Args != nil {
		args = *req.Args
	}

	cmd := exec.CommandContext(cmdCtx, req.Command, args...)
	cmd.Env = os.Environ() // Start with current env
	if req.Env != nil {
		for k, v := range *req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Error("command execution failed", "command", req.Command, "error", err)
		errMsg := err.Error()
		// We return 200 OK with exit code and error message as per spec/implementation choice in previous step
		// But wait, the spec defines 500 for internal error, but for "execution failed" (non-zero exit), 
		// we usually want to return the exit code in the successful response object if the *invocation* succeeded.
		
		return ExecuteCommand200JSONResponse{
			ExitCode: cmd.ProcessState.ExitCode(),
			Output:   string(output),
			Error:    &errMsg,
		}, nil
	}

	return ExecuteCommand200JSONResponse{
		ExitCode: 0,
		Output:   string(output),
	}, nil
}

