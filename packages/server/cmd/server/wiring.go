package main

import (
	"database/sql"

	"github.com/cw-tc/server/internal/api"
	"github.com/cw-tc/server/internal/config"
	"github.com/cw-tc/server/internal/identity"
)

func buildAPI(cfg *config.Config, database *sql.DB) *api.Server {
	identityService := identity.NewService(database, []byte(cfg.IDCardEncKey), []byte(cfg.IDCardHMACKey))
	return api.NewWithIdentityAndDatabase(cfg, identityService, database)
}
