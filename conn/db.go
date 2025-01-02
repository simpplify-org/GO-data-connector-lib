package conn

import (
	"database/sql"
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
}

func NewConn(config Config) (*sql.DB, error) {
	var db *sql.DB
	dsn := config.DBDriver + "://" + config.DBUser + ":" + config.DBPassword + "@" + config.DBHost + ":" + config.DBPort + "/" + config.DBDatabase + config.DBSSLMode

	db, err := sql.Open(config.DBDriver, dsn)
	if err != nil {
		log.Fatal("Cannot connect to db:", err)
		return nil, err
	}

	return db, nil
}
