package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"backend/internal/auth"
	"backend/internal/config"
	"backend/internal/database"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load("../../.env")
	cfg := config.Load()
	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}

	repo := auth.NewRepository(db)
	user, _, err := repo.FindByUsername(context.Background(), "ridwan")
	if err != nil {
		log.Fatal(err)
	}

	b, _ := json.MarshalIndent(user, "", "  ")
	fmt.Println(string(b))
}
