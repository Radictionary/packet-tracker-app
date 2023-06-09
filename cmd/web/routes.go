package main

import (
	"net/http"

	"github.com/Radictionary/website/pkg/config"
	"github.com/Radictionary/website/pkg/handlers"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func routes(app *config.AppConfig) http.Handler {
	mux := chi.NewRouter()
	if app.InProduction {
		mux.Use(middleware.Recoverer)
		mux.Use(middleware.Logger)
	} else {
		mux.Use(middleware.Heartbeat("/ping"))
		mux.Use(middleware.NoCache)
	}

	mux.Use(middleware.CleanPath)

	mux.Get("/", handlers.Repo.Home)
	mux.Get("/packet", handlers.Repo.SseHandler) //communicate with SSE
	mux.Get("/packetinfo", handlers.Repo.SearchPacket) //Search the backend for packet info
	mux.Get("/settings", handlers.Repo.SettingsSync) //Communicate to the frontend on set settings(filters, interface, etc)
	mux.Post("/change", handlers.Repo.Change) // Frontend communicates to the backend on changing settings(filters, interface, etc)
	mux.Post("/upload", handlers.Repo.Upload) //Upload a pcap file to the backend

	fileServer := http.FileServer(http.Dir("./frontend/"))
	mux.Handle("/frontend/*", http.StripPrefix("/frontend", fileServer))

	return mux
}
