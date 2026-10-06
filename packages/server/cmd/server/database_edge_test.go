package main

import (
	"context"
	"testing"

	"github.com/cw-tc/server/internal/config"
)

func TestOpenDevelopmentDatabaseCloseIsIdempotent(t *testing.T) {
	cfg := &config.Config{DeployEnv: "development", DBDriver: "sqlite"}
	_, closeDB, err := openDatabase(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	closeDB()
	closeDB()
}
