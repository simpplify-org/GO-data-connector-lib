package conn

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

type Config struct {
	DBDriver        string
	DBUser          string
	DBPassword      string
	DBHost          string
	DBPort          string
	DBDatabase      string
	DBSSLMode       string
	AppName         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func NewConn(config Config) (*sql.DB, error) {
	var db *sql.DB

	config.validate()

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
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	return db, nil
}

func (c *Config) validate() {
	if c.AppName == "" {
		c.AppName = "data-connector-lib"
	}
	if c.MaxOpenConns <= 0 {
		c.MaxOpenConns = 20
	}
	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = 10
	}
	if c.ConnMaxLifetime <= 0 {
		c.ConnMaxLifetime = time.Hour
	}
}
