package main

import (
	"encoding/json"
	"fmt"
	"github.com/bobby-lin/chirpy/internal/database"
	"github.com/bobby-lin/chirpy/internal/security"
	"github.com/bobby-lin/chirpy/internal/utils"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

type apiConfig struct {
	fileserverHits               int
	db                           *database.DB
	accessTokenExpiresInSeconds  int
	refreshTokenExpiresInSeconds int
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
		return
	}

	dbConn, err := database.NewDB("./database.json")
	if err != nil {
		log.Fatal(err)
		return
	}

	apiCfg := apiConfig{
		db:                           dbConn,
		accessTokenExpiresInSeconds:  60 * 60,           // 1 hour
		refreshTokenExpiresInSeconds: 60 * 60 * 24 * 60, // 60 days
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

// Create API sub-routes
func apiRouter(apiCfg *apiConfig) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", handlerReadiness)
	r.HandleFunc("/reset", apiCfg.handlerReset)

	r.Post("/validate_chirp", handlerValidateChirp)
	r.Get("/chirps", apiCfg.handlerGetChirps)
	r.Get("/chirps/{chirpID}", apiCfg.handlerGetChirp)
	r.Post("/chirps", apiCfg.handlerPostChirps)
	r.Delete("/chirps/{chirpID}", apiCfg.handlerDeleteChirp)

	r.Post("/users", apiCfg.handlerPostUsers)
	r.Post("/login", apiCfg.handlerPostLogin)
	r.Put("/users", apiCfg.handlerUpdateUsers)

	r.Post("/refresh", apiCfg.handlerRefreshToken)
	r.Post("/revoke", apiCfg.handlerRevokeRefreshToken)

	r.Post("/polka/webhooks", apiCfg.handlerWebhook)

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

func (cfg *apiConfig) handlerDeleteChirp(w http.ResponseWriter, r *http.Request) {
	token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
	claims, err := security.GetTokenClaims(token)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	issuer, err := claims.GetIssuer()
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	if issuer != "chirpy-access" {
		respondWithError(w, http.StatusUnauthorized, "action requires an access token")
		return
	}

	id, err := claims.GetSubject()
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "user id is invalid")
		return
	}

	userID, err := strconv.Atoi(id)

	paramValue := chi.URLParam(r, "chirpID")
	chirpID, err := strconv.Atoi(paramValue)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid chirp id value: "+paramValue)
		return
	}

	statusCode, err := cfg.db.DeleteChirps(userID, chirpID)
	if err != nil {
		respondWithError(w, statusCode, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (cfg *apiConfig) handlerGetChirp(w http.ResponseWriter, r *http.Request) {
	chirpID := chi.URLParam(r, "chirpID")

	id, err := strconv.Atoi(chirpID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid chirp id value: "+chirpID)
		return
	}

	c, err := cfg.db.GetChirp(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		//respondWithError(w, http.StatusNotFound, "fail to get chirp with id "+chirpID)
		return
	}
	file, _ := json.Marshal(c)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(file))

}

func (cfg *apiConfig) handlerGetChirps(w http.ResponseWriter, r *http.Request) {
	paramAuthorID := r.URL.Query().Get("author_id")

	if paramAuthorID == "" {
		paramAuthorID = "0"
	}

	authorID, err := strconv.Atoi(paramAuthorID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	paramSortOrder := r.URL.Query().Get("sort")

	sortOrder := "asc"
	if paramSortOrder == "desc" {
		sortOrder = "desc"
	}

	chirpsList, err := cfg.db.GetChirps(authorID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "fail to get chirps")
		return
	}

	if sortOrder == "asc" {
		sort.Slice(chirpsList, func(i, j int) bool {
			return chirpsList[i].ID < chirpsList[j].ID
		})
	} else {
		sort.Slice(chirpsList, func(i, j int) bool {
			return chirpsList[i].ID > chirpsList[j].ID
		})
	}

	file, err := json.Marshal(chirpsList)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(file)
}

func (cfg *apiConfig) handlerPostChirps(w http.ResponseWriter, r *http.Request) {
	token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
	claims, err := security.GetTokenClaims(token)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	issuer, err := claims.GetIssuer()
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	if issuer != "chirpy-access" {
		respondWithError(w, http.StatusUnauthorized, "action requires an access token")
		return
	}

	id, err := claims.GetSubject()
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "user id is invalid")
		return
	}

	userId, err := strconv.Atoi(id)

	type requestBody struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	reqBody := requestBody{}
	err = decoder.Decode(&reqBody)

	c, err := cfg.db.CreateChirp(reqBody.Body, userId)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "fail to create chirp")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	file, err := json.Marshal(c)
	w.Write(file)
}

func (cfg *apiConfig) handlerPostUsers(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	reqBody := requestBody{}
	err := decoder.Decode(&reqBody)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "fail to create user")
		return
	}

	user, err := cfg.db.CreateUser(reqBody.Email, reqBody.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "fail to create user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	file, _ := json.Marshal(user)
	w.Write(file)

}

func (cfg *apiConfig) handlerPostLogin(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	reqBody := requestBody{}
	err := decoder.Decode(&reqBody)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "fail to login")
		return
	}

	email := reqBody.Email
	password := reqBody.Password

	user, err := cfg.db.GetUser(email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "fail to login")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	accessToken, err := security.CreateJwtToken(user.ID, cfg.accessTokenExpiresInSeconds, "chirpy-access")
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "fail to generate accessToken")
		return
	}

	refreshToken, err := security.CreateJwtToken(user.ID, cfg.refreshTokenExpiresInSeconds, "chirpy-refresh")
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "fail to generate refreshToken")
		return
	}

	type responseBody struct {
		Id           int    `json:"id"`
		Email        string `json:"email"`
		IsChirpyRed  bool   `json:"is_chirpy_red"`
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := responseBody{
		Id:           user.ID,
		Email:        user.Email,
		IsChirpyRed:  user.IsChirpyRed,
		Token:        accessToken,
		RefreshToken: refreshToken,
	}

	file, _ := json.Marshal(resp)
	w.Write(file)
}

func (cfg *apiConfig) handlerUpdateUsers(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Email    string `json:"email,omitempty"`
		Password string `json:"password,omitempty"`
	}

	decoder := json.NewDecoder(r.Body)
	reqBody := requestBody{}
	err := decoder.Decode(&reqBody)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "fail to update user")
		return
	}

	token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
	claims, err := security.GetTokenClaims(token)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	issuer, err := claims.GetIssuer()
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	if issuer == "chirpy-refresh" {
		respondWithError(w, http.StatusUnauthorized, "cannot use refresh token to perform the action")
		return
	}

	userId, err := claims.GetSubject()
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "user id is invalid")
		return
	}

	id, err := strconv.Atoi(userId)
	user, err := cfg.db.UpdateUser(id, reqBody.Email, reqBody.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "fail to update user")
		return
	}

	user.Password = "" // Remove password from request :)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	dat, _ := json.Marshal(user)
	w.Write(dat)
}

func (cfg *apiConfig) handlerRefreshToken(w http.ResponseWriter, r *http.Request) {
	token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
	claims, err := security.GetTokenClaims(token)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	issuer, err := claims.GetIssuer()
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	if issuer != "chirpy-refresh" {
		respondWithError(w, http.StatusUnauthorized, "action requires a refresh token")
		return
	}

	isRevoked, err := cfg.db.CheckTokenRevocation(token)

	if isRevoked {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	userId, err := claims.GetSubject()
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "user id is invalid")
		return
	}

	id, err := strconv.Atoi(userId)

	type responseBody struct {
		Token string `json:"token"`
	}

	accessToken, err := security.CreateJwtToken(id, cfg.accessTokenExpiresInSeconds, "chirpy-access")
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "user id is invalid")
		return
	}

	respBody := responseBody{
		Token: accessToken,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	dat, _ := json.Marshal(respBody)
	w.Write(dat)
}

func (cfg *apiConfig) handlerRevokeRefreshToken(w http.ResponseWriter, r *http.Request) {
	token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
	claims, err := security.GetTokenClaims(token)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	issuer, err := claims.GetIssuer()
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "token is invalid")
		return
	}

	if issuer != "chirpy-refresh" {
		respondWithError(w, http.StatusUnauthorized, "action requires a refresh token")
		return
	}

	isRevoked, err := cfg.db.CheckTokenRevocation(token)

	if isRevoked {
		respondWithError(w, http.StatusUnauthorized, "token is already revoked")
		return
	}

	err = cfg.db.RevokeRefreshToken(token)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "fail to revoke refresh token")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (cfg *apiConfig) handlerWebhook(w http.ResponseWriter, r *http.Request) {
	requestApiKey := r.Header.Get("Authorization")
	if requestApiKey != "ApiKey "+os.Getenv("POLKA_API_KEY") {
		respondWithError(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	type requestBody struct {
		Event string `json:"event"`
		Data  struct {
			UserID int `json:"user_id"`
		} `json:"data"`
	}

	decoder := json.NewDecoder(r.Body)
	reqBody := requestBody{}
	err := decoder.Decode(&reqBody)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "fail to update user")
		return
	}

	if reqBody.Event != "user.upgraded" {
		w.WriteHeader(http.StatusOK)
		return
	}

	status, err := cfg.db.UpdateChirpyRedStatus(reqBody.Data.UserID)
	if err != nil {
		log.Println(err)
	}

	w.WriteHeader(status)
}
