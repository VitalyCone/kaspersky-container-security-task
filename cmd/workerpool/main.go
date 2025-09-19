package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/VitalyCone/kaspersky-container-security-task/internal/app"
	"github.com/VitalyCone/kaspersky-container-security-task/internal/config"
)

func main() {
	config := config.MustLoad()

	app, err := app.New(config)
	if err != nil {
		log.Panicf("Failed to create app: %v", err)
	}
	
	go func() {
		if err := app.APIServer.Run(); err != nil{
			log.Panicf("Failed to start API server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<- stop
	app.APIServer.Stop(context.Background())
	log.Println("stop gratefully")
}

