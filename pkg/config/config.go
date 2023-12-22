package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func GetConnectionString() (string, error) {
	if err := godotenv.Load(); err != nil {
		return "", err
	}

	connStr := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"), os.Getenv("DB_SLL_MODE"))

	return connStr, nil
}
