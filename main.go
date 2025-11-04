package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	const port = "8080"

	ServeMux := http.NewServeMux()
	Server := http.Server{
		Handler: ServeMux,
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
	log.Fatal(Server.ListenAndServe())
}
