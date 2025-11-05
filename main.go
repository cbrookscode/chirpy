package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (a *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (a *apiConfig) handlerMetrics(reswrit http.ResponseWriter, req *http.Request) {
	reswrit.Header().Add("Content-Type", "text/plain; charset=utf-8")
	reswrit.WriteHeader(http.StatusOK)
	new := fmt.Sprintf("Hits: %v\n", a.fileserverHits.Load())
	reswrit.Write([]byte(new))
}

func (a *apiConfig) handlerReset(reswrit http.ResponseWriter, req *http.Request) {
	reswrit.Header().Add("Content-Type", "text/plain; charset=utf-8")
	reswrit.WriteHeader(http.StatusOK)
	a.fileserverHits.Swap(0)
	new := fmt.Sprintf("Coutner has been reset: %v\n", a.fileserverHits.Load())
	reswrit.Write([]byte(new))
}

func main() {
	const filepathroot = "."
	const port = "8080"

	api := &apiConfig{}

	srvmux := http.NewServeMux()
	srvmux.Handle("/app/", api.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(filepathroot)))))
	srvmux.HandleFunc("/healthz", handlerReadiness)
	srvmux.HandleFunc("/metrics", api.handlerMetrics)
	srvmux.HandleFunc("/reset", api.handlerReset)

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

func handlerReadiness(reswrit http.ResponseWriter, req *http.Request) {
	reswrit.Header().Add("Content-Type", "text/plain; charset=utf-8")
	reswrit.WriteHeader(http.StatusOK)
	reswrit.Write([]byte("OK"))
}
