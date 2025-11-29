package encv

import (
	"context"

	"github.com/Soltus/encv-go/internal/server"
)

func StartWebdav(ctx context.Context) (string, string, error) {
	return server.StartWebdav(ctx)
}
