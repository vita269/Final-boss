package db

import (
	"database/sql"
	"os"

	_ "modernc.org/sqlite"
)

var scheduler *sql.DB

const schema = `
CREATE TABLE IF NOT EXISTS scheduler (
id INTEGER PRIMARY KEY AUTOINCREMENT,
date CHAR(8) NOT NULL DEFAULT "",
title VARCHAR(255) NOT NULL DEFAULT "",
comment TEXT,
repeat VARCHAR(128)
);

CREATE INDEX IF NOT EXISTS idx_scheduler_date ON scheduler(date);

`

func Init(dbFile string) error {
	if dbFile == "" {
		envDB := os.Getenv("TODO_DBFILE")
		if envDB != "" {
			dbFile = envDB
		} else {
			dbFile = "scheduler.db"
		}
	}
	_, err := os.Stat(dbFile)

	var install bool
	if err != nil {
		install = true
	}

	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		return err
	}
	if _, err := db.Exec("SELECT 1"); err != nil { // Проверка подключения
		return err
	}

	scheduler = db

	if install {
		_, err = db.Exec(schema)
		if err != nil {
			db.Close()
			scheduler = nil
			return err
		}
	}
	return nil
}
func Close() error {
	if scheduler != nil {
		err := scheduler.Close()
		scheduler = nil
		return err
	}
	return nil
}

func GetDB() *sql.DB {
	return scheduler
}
