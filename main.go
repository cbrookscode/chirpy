package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/cbrookscode/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	const filepathroot = "."
	const port = "8080"

	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Printf("error opening db: %v", err)
		return
	}
	dbQueries := database.New(db)

	myplatform := os.Getenv("PLATFORM")
	cfg := &apiConfig{db: dbQueries, platform: myplatform}

	srvmux := http.NewServeMux()
	srvmux.Handle("/app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(filepathroot)))))
	srvmux.HandleFunc("GET /api/healthz", handlerReadiness)
	srvmux.HandleFunc("GET /admin/metrics", cfg.handlerMetrics)
	srvmux.HandleFunc("POST /admin/reset", cfg.handlerReset)
	srvmux.HandleFunc("POST /api/chirps", cfg.handlerChirps)
	srvmux.HandleFunc("POST /api/users", cfg.handlerCreateUser)
	srvmux.HandleFunc("GET /api/chirps", cfg.handlerGetChirps)
	srvmux.HandleFunc("GET /api/chirps/{chirpID}", cfg.handlerGetSingleChirp)
	srvmux.HandleFunc("POST /api/login", cfg.handlerValidateUser)

	srv := http.Server{
		Handler: srvmux,
		Addr:    ":" + port,
	}

	// create log file to write all server logs to
	logfile, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("issue opening log file: %v", err)
	}
	defer logfile.Close()

	// direct log outputs to specified file
	log.SetOutput(logfile)

	log.Printf("serving on port %v\n", port)
	log.Fatal(srv.ListenAndServe())
}
