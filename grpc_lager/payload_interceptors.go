package grpc_lager

import (
	"context"

	"github.com/Unity-Technologies/go-lager-internal"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var (
	// JSONPbFormatter is the formatter used for formatting protobuf messages as strings.
	// If needed, this variable can be reassigned with a different formatter with the same Format() signature.
	JSONPbFormatter JSONPbFormater = &protojson.MarshalOptions{}
)

// JSONPbFormater is a formatter that formats protobuf messages.
type JSONPbFormater interface {
	Format(m proto.Message) string
}

// ServerPayloadLoggingDecider is a user-provided function for deciding whether to log the server-side
// request/response payloads
type ServerPayloadLoggingDecider func(ctx context.Context, fullMethodName string, servingObject interface{}) bool

func PayloadUnaryServerInterceptor(decider ServerPayloadLoggingDecider) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !decider(ctx, info.FullMethod, info.Server) {
			return handler(ctx, req)
		}

		loggerCtx := lager.ContextPairs(TagsToPairs(ctx)).Merge(serverCallFields(info.FullMethod)).InContext(ctx)
		logEntry := lager.Acc(loggerCtx)
		logProtoMessageAsJSON(logEntry, req, "grpc.request.content", "server request payload logged as grpc.request.content field")
		resp, err := handler(ctx, req)
		if err == nil {
			logProtoMessageAsJSON(logEntry, resp, "grpc.response.content", "server response payload logged as grpc.response.content field")
		}

		return resp, err
	}
}

func logProtoMessageAsJSON(logger lager.Lager, pbMsg interface{}, key string, msg string) {
	if p, ok := pbMsg.(proto.Message); ok {
		logger.MMap(msg, key, JSONPbFormatter.Format(p))
	}
}
