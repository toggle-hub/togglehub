package main

import (
	"log"
	"os"

	"github.com/Roll-Play/togglelabs/pkg/api"
	"github.com/Roll-Play/togglelabs/pkg/storage"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Panic(err)
	}

	storage, err := storage.GetInstance()
	if err != nil {
		log.Panic(err)
	}

	if err := storage.Init(); err != nil {
		log.Panic(err)
	}

	app := api.NewApp(os.Getenv("PORT"), storage)

	log.Panic(app.Listen())
}
