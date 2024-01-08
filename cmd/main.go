package main

import (
	"log"
	"os"

	"github.com/Roll-Play/togglelabs/pkg/api"
	"github.com/Roll-Play/togglelabs/pkg/api/common"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/storage"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Panic(err)
	}

	config.ConfigEnvironment()

	storage, err := storage.GetInstance()
	if err != nil {
		log.Panic(err)
	}

	if err := storage.Init(); err != nil {
		log.Panic(err)
	}

	logger, err := common.NewZapLogger()
	if err != nil {
		log.Panic(err)
	}

	app := api.NewApp(os.Getenv("PORT"), storage, logger)

	log.Panic(app.Listen())
}
