package conn

import (
	"database/sql"
	"fmt"
	"log"
)

type Config struct {
	DBDriver   string
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     string
	DBDatabase string
	DBSSLMode  string
	AppName    string
}

func NewConn(config Config) (*sql.DB, error) {
	var db *sql.DB

	if config.AppName == "" {
		config.AppName = "data-connector-lib"
	}
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s application_name=%s",
		config.DBHost,
		config.DBPort,
		config.DBUser,
		config.DBPassword,
		config.DBDatabase,
		config.DBSSLMode,
		config.AppName,
	)

	db, err := sql.Open(config.DBDriver, dsn)
	if err != nil {
		log.Fatal("Cannot connect to db:", err)
		return nil, err
	}

	return db, nil
}
