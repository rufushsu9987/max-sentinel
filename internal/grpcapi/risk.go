package grpcapi

import (
	"context"
	"strings"
	"time"

	"github.com/rufushsu9987/max-sentinel/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type SnapshotProvider interface {
	Snapshot(market string, now time.Time) (domain.RiskSnapshot, bool)
}

type RiskServiceServer interface {
	Evaluate(context.Context, *structpb.Struct) (*structpb.Struct, error)
}

type RiskService struct {
	provider SnapshotProvider
}

func NewRiskService(provider SnapshotProvider) *RiskService {
	return &RiskService{provider: provider}
}

func (s *RiskService) Evaluate(_ context.Context, request *structpb.Struct) (*structpb.Struct, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	marketValue, ok := request.Fields["market"]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "market is required")
	}
	market := strings.ToLower(strings.TrimSpace(marketValue.GetStringValue()))
	if market == "" {
		return nil, status.Error(codes.InvalidArgument, "market is required")
	}
	snapshot, exists := s.provider.Snapshot(market, time.Now().UTC())
	if !exists {
		return nil, status.Errorf(codes.NotFound, "market %q has no state", market)
	}
	response, err := structpb.NewStruct(map[string]any{
		"market":              snapshot.Market,
		"observed_at":         snapshot.ObservedAt.Format(time.RFC3339Nano),
		"mid_price":           snapshot.MidPrice,
		"spread_bps":          snapshot.SpreadBPS,
		"book_imbalance":      snapshot.BookImbalance,
		"realized_volatility": snapshot.RealizedVol,
		"feed_age_seconds":    snapshot.FeedAgeSeconds,
		"severity":            string(snapshot.Severity),
		"reasons":             stringsToAny(snapshot.Reasons),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build response: %v", err)
	}
	return response, nil
}

func stringsToAny(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func RegisterRiskServiceServer(server grpc.ServiceRegistrar, service RiskServiceServer) {
	server.RegisterService(&RiskService_ServiceDesc, service)
}

func _RiskService_Evaluate_Handler(service any, ctx context.Context, decoder func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	request := new(structpb.Struct)
	if err := decoder(request); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return service.(RiskServiceServer).Evaluate(ctx, request)
	}
	info := &grpc.UnaryServerInfo{Server: service, FullMethod: "/maxsentinel.risk.v1.RiskService/Evaluate"}
	handler := func(ctx context.Context, req any) (any, error) {
		return service.(RiskServiceServer).Evaluate(ctx, req.(*structpb.Struct))
	}
	return interceptor(ctx, request, info, handler)
}

var RiskService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "maxsentinel.risk.v1.RiskService",
	HandlerType: (*RiskServiceServer)(nil),
	Methods: []grpc.MethodDesc{{
		MethodName: "Evaluate",
		Handler:    _RiskService_Evaluate_Handler,
	}},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/risk/v1/risk.proto",
}
