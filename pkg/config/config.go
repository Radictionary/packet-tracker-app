package config

import (
	"fmt"
	"html/template"
	"log"
)

// AppConfig holds the application config
type AppConfig struct {
	UseCache      bool
	TemplateCache map[string]*template.Template
	InfoLog       *log.Logger
	InProduction  bool
}

func Handle(err error, description string, fatal bool) {
	if fatal {
		if err != nil {
			log.Panicf("ERROR: %v:%v\n", description, err)
		}
	} else {
		if err != nil {
			fmt.Printf("ERROR: %v:%v\n", description, err)
		}
	}
}
