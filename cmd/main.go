package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Roll-Play/togglelabs/pkg/api"
	"github.com/Roll-Play/togglelabs/pkg/config"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.DBConnectionTimeout*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("DATABASE_URL")))
	if err != nil {
		log.Panic(err)
	}

	db := client.Database("testing")
	app := api.NewServer(os.Getenv("PORT"), db)

	log.Panic(app.Listen())
}
