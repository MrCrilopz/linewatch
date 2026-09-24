package main

import (
	"log"
	"net/http"

	httpadapter "linewatch/internal/adapter/http"
	"linewatch/internal/adapter/sqlite"
)

func main() {
	db, err := sqlite.Open("linewatch.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Load("data/readings.csv", "data/events.csv"); err != nil {
		log.Fatal(err)
	}
	meters, readings, _, err := db.Counts()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded meters=%d readings=%d", meters, readings)
	addr := ":8080"
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpadapter.NewMux(db)))
}
