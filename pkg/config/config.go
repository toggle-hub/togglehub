package config

import "os"

const (
	DBConnectionTimeout   = 10
	DBFetchTimeout        = 5
	JWTExpireTime         = 60 * 60 * 1000 * 24
	BCryptCost            = 8
	TestDBName            = "togglelabs_test"
	DevEnvironment        = "DEV"
	ProductionEnvironment = "PRODUCTION"
)

var Environment string

func StartEnvironment() {
	env := os.Getenv("ENV")

	if env == ProductionEnvironment {
		Environment = ProductionEnvironment
		return
	}

	Environment = DevEnvironment
}
