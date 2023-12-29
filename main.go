package main

import (
	"encoding/json"
	"fmt"
	"github.com/bobby-lin/chirpy/internal/database"
	"github.com/bobby-lin/chirpy/internal/utils"
	"github.com/go-chi/chi/v5"
	"log"
	"net/http"
	"sort"
	"strconv"
)

type apiConfig struct {
	fileserverHits int
	db             *database.DB
}

func main() {
	dbConn, err := database.NewDB("./database.json")
	if err != nil {
		log.Fatal(err)
		return
	}

	apiCfg := apiConfig{
		db: dbConn,
	}

	r := chi.NewRouter()
	r.Handle("/app", http.StripPrefix("/app", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir("./app")))))
	r.Handle("/app/*", http.StripPrefix("/app/assets/", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir("./app/assets/")))))
	r.Mount("/api", apiRouter(&apiCfg))
	r.Mount("/admin", adminRouter(&apiCfg))

	corsRouter := middlewareCors(r)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: corsRouter,
	}

	log.Print("Serving on port: 8080")
	log.Fatal(srv.ListenAndServe())
}

func adminRouter(apiCfg *apiConfig) http.Handler {
	r := chi.NewRouter()
	r.Get("/metrics", apiCfg.handlerMetric)
	return r
}

// Create api sub-routes
func apiRouter(apiCfg *apiConfig) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", handlerReadiness)
	r.HandleFunc("/reset", apiCfg.handlerReset)
	r.Post("/validate_chirp", handlerValidateChirp)
	r.Get("/chirps", apiCfg.handlerGetChirps)
	r.Post("/chirps", apiCfg.handlerPostChirps)
	return r
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
	w.Header().Set("Content-Type", "text/html")

	page := fmt.Sprintf("<html>\n\n<body>\n    <h1>Welcome, Chirpy Admin</h1>\n    <p>Chirpy has been visited %d times!</p>\n</body>\n\n</html>\n", cfg.fileserverHits)

	_, err := w.Write([]byte(page))

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

func handlerValidateChirp(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	reqBody := requestBody{}
	err := decoder.Decode(&reqBody)

	if err != nil {
		log.Printf("Error decoding request body: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	if len(reqBody.Body) > 140 {
		log.Printf("Chirp is too long")
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	filteredBody, _ := utils.FilterWords(reqBody.Body)

	respondWithJSON(w, http.StatusOK, filteredBody)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type responseBody struct {
		Error string `json:"error"`
	}

	respBody := responseBody{
		Error: msg,
	}
	dat, _ := json.Marshal(respBody)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	type responseBody struct {
		Body        interface{} `json:"body,omitempty"`
		CleanedBody interface{} `json:"cleaned_body,omitempty"`
	}

	respBody := responseBody{
		CleanedBody: payload,
	}

	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func (cfg *apiConfig) handlerGetChirps(w http.ResponseWriter, r *http.Request) {
	chirpsList, err := cfg.db.GetChirps()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "fail to get chirps")
		return
	}

	sort.Slice(chirpsList, func(i, j int) bool {
		return chirpsList[i].ID < chirpsList[j].ID
	})

	file, err := json.Marshal(chirpsList)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(file)
}

func (cfg *apiConfig) handlerPostChirps(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	reqBody := requestBody{}
	err := decoder.Decode(&reqBody)

	c, err := cfg.db.CreateChirp(reqBody.Body)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "fail to create chirp")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	file, err := json.Marshal(c)
	w.Write(file)
}
