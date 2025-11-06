package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
	reswrit.Header().Add("Content-Type", "text/html; charset=utf-8")
	reswrit.WriteHeader(http.StatusOK)
	new := fmt.Sprintf(
		`<html>
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
		</html>`, a.fileserverHits.Load())
	reswrit.Write([]byte(new))
}

func (a *apiConfig) handlerReset(reswrit http.ResponseWriter, req *http.Request) {
	reswrit.Header().Add("Content-Type", "text/plain; charset=utf-8")
	reswrit.WriteHeader(http.StatusOK)
	a.fileserverHits.Store(0)
	new := fmt.Sprintf("Counter has been reset: %v\n", a.fileserverHits.Load())
	reswrit.Write([]byte(new))
}

func main() {
	const filepathroot = "."
	const port = "8080"

	api := &apiConfig{}

	srvmux := http.NewServeMux()
	srvmux.Handle("/app/", api.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(filepathroot)))))
	srvmux.HandleFunc("GET /api/healthz", handlerReadiness)
	srvmux.HandleFunc("GET /admin/metrics", api.handlerMetrics)
	srvmux.HandleFunc("POST /admin/reset", api.handlerReset)
	srvmux.HandleFunc("POST /api/validate_chirp", handlerValidateChirp)

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

func respondWithError(w http.ResponseWriter, msg string, code int, err error) {
	if err != nil {
		log.Println(err)
	}
	if code > 499 {
		log.Printf("Responding with 5XX error: %v\n", msg)
	}

	type errorResp struct {
		Error string `json:"error"`
	}
	payload := errorResp{Error: msg}

	respondWithJson(w, code, payload)
}

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")

	bytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling json: %v\n", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(bytes)
}

func filterProfanity(text string) string {
	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}

	words := strings.Fields(text)
	for i, word := range words {
		if _, ok := badWords[strings.ToLower(word)]; ok {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
}

func handlerReadiness(resWriter http.ResponseWriter, req *http.Request) {
	resWriter.Header().Add("Content-Type", "text/plain; charset=utf-8")
	resWriter.WriteHeader(http.StatusOK)
	resWriter.Write([]byte("OK"))
}

func handlerValidateChirp(resWriter http.ResponseWriter, req *http.Request) {
	type incoming struct {
		Body string `json:"body"`
	}

	type returnVals struct {
		CleanedBody string `json:"cleaned_body"`
	}

	chirp := incoming{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&chirp)
	if err != nil {
		log.Printf("Error decoding json data in POST request: %v\n", err)
		respondWithError(resWriter, "Something went wrong", 500, err)
		return
	}

	filteredChirp := filterProfanity(chirp.Body)

	if len(filteredChirp) <= 140 {
		// is valid
		payload := returnVals{CleanedBody: filteredChirp}
		respondWithJson(resWriter, 200, payload)
	} else {
		// is not valid
		respondWithError(resWriter, "Chrip is too long", 400, nil)
	}
}
