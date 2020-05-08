package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	expand "github.com/openvenues/gopostal/expand"
	parser "github.com/openvenues/gopostal/parser"
)

type Request struct {
	Query string `json:"query"`
}

// Middleware function, which will be called for each request
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")

		// "Bearer a", the minimal header value, is 8 characters
		if len(token) < 8 {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		tokenBytes := []byte(token[7:])
		keyBytes := []byte(os.Getenv("API_KEY"))

		keyIsValid := subtle.ConstantTimeCompare(tokenBytes, keyBytes) == 1

		if keyIsValid {
			// Pass down the request to the next middleware (or final handler)
			next.ServeHTTP(w, r)
		} else {
			// Write an error and stop the handler chain
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	})
}

func main() {
	host := os.Getenv("LISTEN_HOST")
	port := os.Getenv("LISTEN_PORT")
	if port == "" {
		port = "8080"
	}
	listenSpec := fmt.Sprintf("%s:%s", host, port)

	certFile := os.Getenv("SSL_CERT_FILE")
	keyFile := os.Getenv("SSL_KEY_FILE")

	router := mux.NewRouter()
	router.HandleFunc("/health", HealthHandler).Methods("GET")
	router.HandleFunc("/expand", ExpandHandler).Methods("POST")
	router.HandleFunc("/parser", ParserHandler).Methods("POST")
	router.Use(authMiddleware)

	s := &http.Server{Addr: listenSpec, Handler: router}
	l, err := net.Listen("tcp", listenSpec)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if certFile != "" && keyFile != "" {
			log.Printf("listening on https://%s\n", listenSpec)
			log.Fatal(s.ServeTLS(l, certFile, keyFile))
		} else {
			log.Printf("listening on http://%s\n", listenSpec)
			log.Fatal(s.Serve(l))
		}
	}()

	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt)

	<-stop
	log.Println("\nShutting down the server...")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	s.Shutdown(ctx)
	l.Close()
	log.Println("Server stopped")
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func ExpandHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req Request

	q, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(q, &req)

	expansions := expand.ExpandAddress(req.Query)

	expansionThing, _ := json.Marshal(expansions)
	w.Write(expansionThing)
}

func ParserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req Request

	q, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(q, &req)

	parsed := parser.ParseAddress(req.Query)
	parseThing, _ := json.Marshal(parsed)
	w.Write(parseThing)
}
