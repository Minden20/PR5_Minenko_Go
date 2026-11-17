package main

import (
	"log"
	"net/http"
	"time"

	"pr5_minenko/internal/config"
	"pr5_minenko/internal/database"
	"pr5_minenko/internal/handler"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("[GORM] Failed to connect to PostgreSQL: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("[GORM] Auto-migration failed: %v", err)
	}

	r := handler.NewRouter(db, cfg)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	log.Printf("[СЕРВЕР] Сервіс успішно запущено на http://localhost:%s", cfg.HTTPPort)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("[СЕРВЕР] Помилка: %v", err)
	}
}
