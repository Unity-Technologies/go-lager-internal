package grpc_lager_test

import (
	"context"
	"testing"
	"time"

	"github.com/Unity-Technologies/go-lager-internal"
	"github.com/Unity-Technologies/go-lager-internal/grpc_lager"
	"github.com/Unity-Technologies/go-tutl-internal"
)

func TestDurationToTimeMillisField(t *testing.T) {
	u := tutl.New(t)
	expectedCtx := lager.Pairs("grpc.time_ms", float32(0.1)).InContext(context.TODO())

	ctx := grpc_lager.DurationToTimeMillisField(time.Microsecond * 100).InContext(context.TODO())

	u.Is(expectedCtx, ctx, "sub millisecond values in context should be correct")
}
