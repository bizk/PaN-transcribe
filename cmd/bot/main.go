package main

import (
	"fmt"
	"log"
	"os"

	"github.com/override/pan-transcribe/internal/config"
)

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Bot configured for %d allowed users\n", len(cfg.Telegram.AllowedUsers))
	fmt.Println("Bot placeholder - implementation pending")
}
