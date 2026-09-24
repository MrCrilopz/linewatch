package main

import (
	"log"
	"net/http"

	httpadapter "linewatch/internal/adapter/http"
)

func main() {
	addr := ":8080"
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpadapter.NewMux()))
}
