package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strings"

	httpadapter "linewatch/internal/adapter/http"
	"linewatch/internal/adapter/llm"
	"linewatch/internal/adapter/sqlite"
	"linewatch/internal/app"
)

func main() {
	if err := loadEnv(".env"); err != nil {
		log.Fatal(err)
	}
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
	log.Fatal(http.ListenAndServe(addr, httpadapter.NewMux(db, app.NewRunner(db, llm.New(os.Getenv("OPENROUTER_API_KEY"))))))
}

func loadEnv(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key != "" && os.Getenv(key) == "" {
			if err := os.Setenv(key, val); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
