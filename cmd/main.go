package main

import (
	"log"
	"os"

	"github.com/Roll-Play/togglelabs/pkg/api"
	"github.com/Roll-Play/togglelabs/pkg/api/sqs_helper"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/Roll-Play/togglelabs/pkg/logger"
	"github.com/Roll-Play/togglelabs/pkg/storage"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Panic(err)
	}

	config.StartEnvironment()

	storage, err := storage.GetInstance()
	if err != nil {
		log.Panic(err)
	}

	if err := storage.Init(); err != nil {
		log.Panic(err)
	}

	logger, err := logger.GetInstance()
	if err != nil {
		log.Panic(err)
	}

	sqsHelper, err := sqs_helper.GetInstance()
	if err != nil {
		log.Panic(err)
	}

	app := api.NewApp(os.Getenv("PORT"), storage, logger, sqsHelper)

	log.Panic(app.Listen())
}
