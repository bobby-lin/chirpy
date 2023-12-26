package main

import (
	"github.com/go-chi/chi/v5"
	"log"
	"net/http"
	"strconv"
)

type apiConfig struct {
	fileserverHits int
}

func main() {
	apiCfg := apiConfig{}
	r := chi.NewRouter()

	//mux := http.NewServeMux()
	//mux.Handle("/app/", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir("./app/")))))
	//mux.Handle("/app/assets/", http.StripPrefix("/app/assets/", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir("./app/assets/")))))
	//mux.HandleFunc("/healthz", handlerReadiness)
	//mux.HandleFunc("/metrics", apiCfg.handlerMetric)
	//mux.HandleFunc("/reset", apiCfg.handlerReset)

	r.Handle("/app", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir("./app")))))
	r.Handle("/app/*", http.StripPrefix("/app/assets/", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir("./app/assets/")))))
	r.Get("/healthz", handlerReadiness)
	r.Get("/metrics", apiCfg.handlerMetric)
	r.HandleFunc("/reset", apiCfg.handlerReset)

	corsRouter := middlewareCors(r)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: corsRouter,
	}

	log.Print("Serving on port: 8080")
	log.Fatal(srv.ListenAndServe())
}

func middlewareCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits += 1
		next.ServeHTTP(w, r)
	})
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(http.StatusText(http.StatusOK)))
	if err != nil {
		log.Fatal(err)
	}
}

func (cfg *apiConfig) handlerMetric(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write([]byte("Hits: " + strconv.Itoa(cfg.fileserverHits)))
	if err != nil {
		log.Fatal(err)
	}
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits = 0
	_, err := w.Write([]byte("Reset count to " + strconv.Itoa(cfg.fileserverHits)))
	if err != nil {
		log.Fatal(err)
	}
}
