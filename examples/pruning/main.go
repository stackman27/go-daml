package main

import (
	"context"
	"os"
	"time"

	"github.com/noders-team/go-daml/pkg/client"
	"github.com/noders-team/go-daml/pkg/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	grpcAddress := os.Getenv("GRPC_ADDRESS")
	if grpcAddress == "" {
		grpcAddress = "localhost:8080"
	}

	bearerToken := os.Getenv("BEARER_TOKEN")
	if bearerToken == "" {
		log.Warn().Msg("BEARER_TOKEN environment variable not set")
	}

	tlsConfig := client.TlsConfig{}

	cl, err := client.NewDamlClient(bearerToken, grpcAddress).
		WithTLSConfig(tlsConfig).
		Build(context.Background())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build DAML client")
	}

	pruneUpTo := time.Now().Add(-24 * time.Hour).UnixMicro()

	pruneReq := &model.PruneRequest{
		PruneUpTo:                 pruneUpTo,
		SubmissionID:              "prune-" + time.Now().Format("20060102150405"),
		PruneAllDivulgedContracts: false,
	}

	log.Info().
		Time("pruneUpTo", time.UnixMicro(pruneUpTo)).
		Int64("offset", pruneUpTo).
		Msg("attempting to prune ledger")

	err = cl.PruningMng.Prune(context.Background(), pruneReq)
	if err != nil {
		log.Warn().Err(err).Msg("prune operation result")
	} else {
		log.Info().Msg("prune operation completed successfully")
	}
}
