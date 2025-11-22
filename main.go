package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/vita269/final-boss/pkg/api"
	"github.com/vita269/final-boss/pkg/db"
)

type Server struct{}

func (s *Server) Run() {

}
func main() {

	server := &Server{}
	server.Run()

	dbFile := os.Getenv("TODO_DBFILE")
	if dbFile == "" {
		dbFile = "scheduler.db"
	}
	err := db.Init("scheduler.db")
	if err != nil {
		log.Fatalf("Ошибка при инициализации базы данных: %v", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Ошибка при закрытии БД: %v", err)
		}
	}()

	api.Init()

	portStr := os.Getenv("TODO_PORT")
	if portStr == "" {
		portStr = "7540"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("Неверный формат порта:%s", portStr)
	}

	if port < 1 || port > 65535 {
		log.Fatalf("Порт должен быть в диапазоне 1–65535: %d", port)

	}
	webDir := "./web"
	fileServer := http.FileServer(http.Dir(webDir))
	http.Handle("/", fileServer)

	address := ":" + portStr

	log.Printf("Сервер запущен на порту %s", portStr)
	log.Printf("Откройте в браузере: http://localhost%s/", portStr)
	log.Printf("Корневая директория: %s", webDir)

	err = http.ListenAndServe(address, nil)
	if err != nil {
		log.Fatalf("Ошибка при запуске сервера: %v", err)
	}

}
