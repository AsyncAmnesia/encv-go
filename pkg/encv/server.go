package encv

import (
	"github.com/Soltus/encv-go/internal/server"
	"github.com/Soltus/encv-go/internal/types"
)

func StartWebdav(cfg *types.UserConfig) (string, string, error) {
	return server.StartWebdav(cfg)
}
