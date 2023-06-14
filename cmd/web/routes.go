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
	} else {
		mux.Use(middleware.Heartbeat("/ping"))
		mux.Use(middleware.NoCache)
		mux.Use(middleware.Logger)
	}

	mux.Use(middleware.CleanPath)

	mux.Get("/", handlers.Repo.Home)
	mux.Get("/packet", handlers.Repo.SseHandler) //Use SSE to send packets to the frontend
	mux.Get("/packetinfo", handlers.Repo.SearchPacket) //Search the backend for packet information
	mux.Get("/settings", handlers.Repo.SettingsSync) //Tell the frontend the settings stored in database(filters, interface, etc)
	mux.Get("/recover", handlers.Repo.Recover) //Recover the packets from DB
	mux.Post("/change", handlers.Repo.Change) // Frontend communicates to the backend on changing settings(filters, interface, etc)
	mux.Post("/upload", handlers.Repo.Upload) //Upload a pcap file to the backend


	fileServer := http.FileServer(http.Dir("./frontend/"))
	mux.Handle("/frontend/*", http.StripPrefix("/frontend", fileServer))

	return mux
}
