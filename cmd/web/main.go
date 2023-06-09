package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/Radictionary/website/pkg/config"
	"github.com/Radictionary/website/pkg/handlers"
	"github.com/Radictionary/website/pkg/render"
)

var portNumber = ":8080"
var app config.AppConfig

// main is the main function
func main() {
	// change this to true when in production
	app.InProduction = false

	tc, err := render.CreateTemplateCache()
	if err != nil {
		log.Fatal("cannot create template cache")
	}

	app.TemplateCache = tc
	app.UseCache = app.InProduction

	repo := handlers.NewRepo(&app)
	handlers.NewHandlers(repo)

	render.NewTemplates(&app)

	fmt.Printf("Staring application on http://localhost%v with app.Inproduction set to %v\n", portNumber, app.InProduction)

	srv := &http.Server{
		Addr:    portNumber,
		Handler: routes(&app),
	}

	err = srv.ListenAndServe()
	if err != nil && strings.Contains(string(err.Error()), "bind: address already in use") {
		fmt.Println("Port 8080 in use, continuing on port 8081")
		srv = &http.Server{
			Addr:    ":8081",
			Handler: routes(&app),
		}
		err = srv.ListenAndServe()
		if err != nil {
			fmt.Println("Tried to bind port to 8081 and still failed")
		}
	} else if err != nil {
		config.Handle(err, "Starting server", true)
	}
}
