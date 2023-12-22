package main

import (
	"log"
	"os"

	"github.com/Roll-Play/togglelabs/pkg/api"
	"github.com/Roll-Play/togglelabs/pkg/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}

	connStr, err := config.GetConnectionString()
	if err != nil {
		log.Fatal(err)
	}

	conn, err := sqlx.Open("pgx", connStr)

	if err != nil {
		conn.Close()
		log.Fatal(err)
	}

	defer conn.Close()
	app := api.NewServer(os.Getenv("PORT"), conn)

	log.Fatal(app.Listen())
}
