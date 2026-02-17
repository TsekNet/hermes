// Package server implements the HermesService gRPC server.
// It delegates lifecycle management to the manager package.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/manager"
	pb "github.com/TsekNet/hermes/proto"
	"github.com/google/deck"
	"google.golang.org/grpc"
)

// DefaultPort is the default TCP port for the gRPC service.
const DefaultPort = 4770

// Server wraps the gRPC server and notification manager.
type Server struct {
	pb.UnimplementedHermesServiceServer
	mgr    *manager.Manager
	grpc   *grpc.Server
	port   int
}

// New creates a Server bound to the given manager and port.
func New(mgr *manager.Manager, port int) *Server {
	s := &Server{
		mgr:  mgr,
		port: port,
		grpc: grpc.NewServer(grpc.MaxRecvMsgSize(128 * 1024)), // 128 KB max inbound message
	}
	pb.RegisterHermesServiceServer(s.grpc, s)
	return s
}

// Serve starts listening on localhost:<port>. Blocks until Stop is called.
func (s *Server) Serve() error {
	lis, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	deck.Infof("gRPC server listening on %s", lis.Addr())
	return s.grpc.Serve(lis)
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop() {
	s.grpc.GracefulStop()
}

// --- RPC implementations ---

func (s *Server) Notify(ctx context.Context, req *pb.NotifyRequest) (*pb.NotifyResponse, error) {
	cfg, err := config.LoadJSON(req.ConfigJson)
	if err != nil {
		return &pb.NotifyResponse{ExitCode: 1, Error: err.Error()}, nil
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return &pb.NotifyResponse{ExitCode: 1, Error: err.Error()}, nil
	}
	if req.Id != "" {
		cfg.ID = req.Id
	}

	id, resultCh := s.mgr.Submit(cfg)
	deck.Infof("gRPC: Notify id=%s blocking for result", id)

	select {
	case <-ctx.Done():
		s.mgr.Cancel(id)
		return &pb.NotifyResponse{ExitCode: 1, Error: "cancelled by client"}, nil
	case result := <-resultCh:
		return &pb.NotifyResponse{
			Value:    result.Value,
			ExitCode: result.ExitCode,
		}, nil
	}
}

func (s *Server) GetUIConfig(_ context.Context, req *pb.GetUIConfigRequest) (*pb.GetUIConfigResponse, error) {
	cfg, ok := s.mgr.GetConfig(req.NotificationId)
	if !ok {
		return nil, fmt.Errorf("notification %s not found", req.NotificationId)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return &pb.GetUIConfigResponse{
		ConfigJson:      data,
		DeferralAllowed: s.mgr.DeferralAllowed(req.NotificationId),
	}, nil
}

func (s *Server) ReportChoice(_ context.Context, req *pb.ReportChoiceRequest) (*pb.ReportChoiceResponse, error) {
	ok := s.mgr.ReportChoice(req.NotificationId, req.Value)
	return &pb.ReportChoiceResponse{Accepted: ok}, nil
}

func (s *Server) Cancel(_ context.Context, req *pb.CancelRequest) (*pb.CancelResponse, error) {
	found := s.mgr.Cancel(req.NotificationId)
	return &pb.CancelResponse{Found: found}, nil
}

func (s *Server) List(_ context.Context, _ *pb.ListRequest) (*pb.ListResponse, error) {
	infos := s.mgr.List()
	var out []*pb.NotificationInfo
	for _, info := range infos {
		ni := &pb.NotificationInfo{
			Id:          info.ID,
			Heading:     info.Heading,
			State:       string(info.State),
			DeferCount:  int32(info.DeferCount),
			CreatedUnix: info.CreatedAt.Unix(),
		}
		if !info.Deadline.IsZero() {
			ni.DeadlineUnix = info.Deadline.Unix()
		}
		out = append(out, ni)
	}
	return &pb.ListResponse{Notifications: out}, nil
}
