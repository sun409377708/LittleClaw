package app

import (
	"context"
	"fmt"

	"littleclaw/internal/config"
)

func mustConfig(ctx context.Context) *config.Config {
	cfg, ok := config.FromContext(ctx)
	if !ok {
		panic(fmt.Sprintf("config missing from context %T", ctx))
	}
	return cfg
}
