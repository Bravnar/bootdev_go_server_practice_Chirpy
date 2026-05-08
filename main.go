package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Bravnar/go_servers_practice/internal/auth"
	"github.com/Bravnar/go_servers_practice/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

/* ***************************************************************************
*	Structures                                                                 *
* ***************************************************************************/

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
	secret         string
	polkaKey       string
}

type User struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Email       string    `json:"email"`
	Token       string    `json:"token,omitempty"`
	IsChirpyRed bool      `json:"is_chirpy_red"`
}

type userUpdateRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserWithRefreshToken struct {
	User
	RefreshToken string `json:"refresh_token"`
}

type chirpRequest struct {
	Text string `json:"body"`
}

type chirpResponse struct {
	ID        uuid.UUID `json:"id"`
	Text      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserID    uuid.UUID `json:"user_id"`
}

/* ***************************************************************************
*	Middleware                                                                 *
* ***************************************************************************/

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

/* ***************************************************************************
*	Admin endpoints                                                            *
* ***************************************************************************/

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		respondErr(w, 403, "not allowed to delete users if not in dev mode")
		return
	}

	if err := cfg.dbQueries.DeleteUsers(r.Context()); err != nil {
		respondErr(w, 500, "Something went wrong")
		return
	}
	cfg.fileserverHits.Store(0)
	respond(w, 200, "reset count and cleared users")
}

func (cfg *apiConfig) handlerWriteMetrics(w http.ResponseWriter, r *http.Request) {
	resp := fmt.Sprintf(
		"<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>",
		cfg.fileserverHits.Load(),
	)
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte(resp))
	if err != nil {
		log.Println("failed to write to response")
	}
}

func handlerHealthz(writer http.ResponseWriter, req *http.Request) {
	writer.Header().Add("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(200)
	_, err := writer.Write([]byte("OK"))
	if err != nil {
		return
	}
}

/* ***************************************************************************
*	User management                                                            *
* ***************************************************************************/

func (cfg *apiConfig) handlerUserCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body     string `json:"email"`
		Password string `json:"password"`
	}

	params, err := decode[parameters](w, r)
	if err != nil {
		return
	}

	params.Password, err = auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("auth module failed to hash password")
		respondErr(w, 400, "bad request")
		return
	}

	toDB := database.CreateUserParams{
		Email:          params.Body,
		HashedPassword: params.Password,
	}

	user, err := cfg.dbQueries.CreateUser(r.Context(), toDB)
	if err != nil {
		log.Printf("Database failed to create a user: %v", err)
		respondErr(w, 422, "Failed to create user")
		return
	}
	respond(w, 201, User{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	})
}

/* ***************************************************************************
*	Chirp management                                                           *
* ***************************************************************************/

// POST /api/chirps **********************************************************

func filterProfanity(text string) string {
	profaneWords := []string{"kerfuffle", "sharbert", "fornax"}
	splitText := strings.Split(text, " ")
	filteredWords := []string{}

	for _, word := range splitText {
		if slices.Contains(profaneWords, strings.ToLower(word)) {
			filteredWords = append(filteredWords, "****")
		} else {
			filteredWords = append(filteredWords, word)
		}
	}
	return strings.Join(filteredWords, " ")
}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, r *http.Request) {
	params, err := decode[chirpRequest](w, r)
	if err != nil {
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("Authentication issue: %v", err)
		respondErr(w, 401, "No valid token received")
		return
	}

	user, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		log.Printf("Validation failed: %v", err)
		respondErr(w, 401, "Unauthorized")
		return
	}

	params.Text = filterProfanity(params.Text)
	if len(params.Text) > 140 {
		log.Printf("%v: attemtped to send an invalid chirp", user)
		respondErr(w, 400, "Chirp is too long")
		return
	}

	chirpParams := database.CreateChirpParams{
		Body:   params.Text,
		UserID: user,
	}

	chirp, err := cfg.dbQueries.CreateChirp(r.Context(), chirpParams)
	if err != nil {
		log.Printf("failed to create chirp: %v", err)
		respondErr(w, 422, "Unprocessable Entity")
		return
	}

	chirpResp := chirpResponse{
		ID:        chirp.ID,
		Text:      chirp.Body,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		UserID:    chirp.UserID,
	}
	respond(w, 201, chirpResp)
}

// GET /api/chirps ***********************************************************

func (cfg *apiConfig) handlerGetChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.dbQueries.GetChirps(r.Context())
	if err != nil {
		log.Printf("failed to fetch chirps: %v", err)
		respondErr(w, 500, "Faile to fetch from database")
		return
	}
	var resp []chirpResponse
	for _, chirp := range chirps {
		resp = append(resp, chirpResponse{
			ID:        chirp.ID,
			Text:      chirp.Body,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			UserID:    chirp.UserID,
		})
	}
	respond(w, 200, resp)
}

// POST /api/chirps/{chirpID} ************************************************

func (cfg *apiConfig) handlerGetChirp(w http.ResponseWriter, r *http.Request) {
	pathValue, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		log.Printf("couldn't parse uuid")
		respondErr(w, 400, "Something wrong happened")
		return
	}

	chirp, err := cfg.dbQueries.GetChirp(r.Context(), pathValue)
	if err != nil {
		log.Printf("couldn't find %s in db: %v", pathValue, err)
		respondErr(w, 404, "Entry not found in db")
		return
	}

	chirpResp := chirpResponse{
		ID:        chirp.ID,
		Text:      chirp.Body,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		UserID:    chirp.UserID,
	}
	respond(w, 200, chirpResp)
}

func (cfg *apiConfig) handlerDeleteChirp(w http.ResponseWriter, r *http.Request) {
	pathValue, err := uuid.Parse(r.PathValue("chirpID"))
	if err != nil {
		log.Printf("couldn't parse uuid")
		respondErr(w, 400, "Something wrong happened")
		return
	}

	accessToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("No token in Authorization header")
		respondErr(w, 401, "must send refresh token to be authorized")
		return
	}

	userID, err := auth.ValidateJWT(accessToken, cfg.secret)
	if err != nil {
		log.Printf("invalid token")
		respondErr(w, 401, "unauthorized with the provided token")
		return
	}

	chirp, err := cfg.dbQueries.GetChirp(r.Context(), pathValue)
	if err != nil {
		respondErr(w, 404, "Chirp not found")
		return
	}

	if chirp.UserID != userID {
		respondErr(w, 403, "Unauthorized to delete chirp which you are not the author of")
		return
	}

	if err := cfg.dbQueries.DeleteChirp(r.Context(), pathValue); err != nil {
		log.Printf("Failed to delete chirp from db")
		respondErr(w, 500, "Failed to delete chirp from db")
		return
	}
	w.WriteHeader(204)
}

/* ***************************************************************************
*	Authentication                                                             *
* ***************************************************************************/

// POST /api/login ***********************************************************

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {
	type login struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	params, err := decode[login](w, r)
	if err != nil {
		return
	}

	exp := 3600

	user, err := cfg.dbQueries.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		log.Printf("Failed to find user by email in db: %v", err)
		respondErr(w, 401, "Incorrect email or password")
		return
	}

	ok, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if !ok || err != nil {
		log.Printf("Bad credentials provided or hashfunction failed: %v", err)
		respondErr(w, 401, "Incorrect email or password")
		return
	}

	jwtToken, err := auth.MakeJWT(
		user.ID,
		cfg.secret,
		time.Duration(exp)*time.Second,
	)
	if err != nil {
		return
	}

	refreshToken := auth.MakeRefreshToken()
	refreshTokenParams := database.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(time.Duration(24*60) * time.Hour),
	}

	_, err = cfg.dbQueries.CreateRefreshToken(
		r.Context(),
		refreshTokenParams,
	)
	if err != nil {
		log.Printf(
			"Failed to create a refresh token db entry: %v",
			err,
		)
		respondErr(w, 500, "Failed to write to DB")
		return
	}

	respond(w, 200, UserWithRefreshToken{
		User: User{
			ID:          user.ID,
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
			Email:       user.Email,
			Token:       jwtToken,
			IsChirpyRed: user.IsChirpyRed,
		},
		RefreshToken: refreshToken,
	})
}

// POST /api/refresh *********************************************************

func (cfg *apiConfig) handlerRefreshToken(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("No token in Authorization header")
		respondErr(w, 401, "must send refresh token to be authorized")
		return
	}

	user, err := cfg.dbQueries.GetUserByRefreshToken(
		r.Context(),
		refreshToken,
	)
	if err != nil {
		log.Printf("database encountered error: %v", err)
		respondErr(w, 401, "token doesn't exist or is revoked")
		return
	}

	newAccessToken, err := auth.MakeJWT(
		user.ID,
		cfg.secret,
		time.Hour,
	)
	if err != nil {
		log.Printf("failed to create new JWT token: %v", err)
		respondErr(w, 500, "failed to create new JWT token")
		return
	}
	respond(w, 200, map[string]string{"token": newAccessToken})
}

func (cfg *apiConfig) handlerRevokeToken(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("No token in Authorization header")
		respondErr(w, 401, "must send refresh token to be authorized")
		return
	}

	err = cfg.dbQueries.RevokeToken(r.Context(), refreshToken)
	if err != nil {
		log.Printf("failed to revoke token: %v", err)
		respondErr(w, 500, "failed to revoke refresh token")
		return
	}
	w.WriteHeader(204)
}

// PUT /api/users ************************************************************

func (cfg *apiConfig) handlerUpdateUsers(w http.ResponseWriter, r *http.Request) {
	userUpdateRequest, err := decode[userUpdateRequest](w, r)
	if err != nil {
		return
	}

	accessToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("No token in Authorization header")
		respondErr(w, 401, "must send refresh token to be authorized")
		return
	}

	userID, err := auth.ValidateJWT(accessToken, cfg.secret)
	if err != nil {
		log.Printf("invalid token")
		respondErr(w, 401, "unauthorized with the provided token")
		return
	}

	hashedPassword, err := auth.HashPassword(userUpdateRequest.Password)
	if err != nil {
		log.Printf("Failed to hash password string")
		respondErr(w, 500, "Failed to hash paswword string")
		return
	}

	userUpdateParams := database.UpdateUserEmailAndPassParams{
		Email:          userUpdateRequest.Email,
		HashedPassword: hashedPassword,
		ID:             userID,
	}

	user, err := cfg.dbQueries.UpdateUserEmailAndPass(r.Context(), userUpdateParams)
	if err != nil {
		log.Printf("failed to update user record in db: %v", err)
		respondErr(w, 500, "failed to update user in the db")
	}

	respond(w, 200, User{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		Token:       accessToken,
		IsChirpyRed: user.IsChirpyRed,
	})
}

/* ***************************************************************************
*	Authentication                                                             *
* ***************************************************************************/

// POST /api/polka/webhooks **************************************************

func (cfg *apiConfig) handlerPolkaWebhook(w http.ResponseWriter, r *http.Request) {
	type DataStruct struct {
		UserID string `json:"user_id"`
	}

	type EventStruct struct {
		Event string     `json:"event"`
		Data  DataStruct `json:"data"`
	}

	apiKey, err := auth.GetAPIToken(r.Header)
	if err != nil {
		log.Printf("malformed apiKey: %v", err)
		respondErr(w, 401, "malformed apiKey")
		return
	}

	if apiKey != cfg.polkaKey {
		log.Printf("apiKey does not match")
		respondErr(w, 401, "unauthorized")
		return
	}

	eventReq, err := decode[EventStruct](w, r)
	if err != nil {
		log.Printf("failed decoding the webhook request: %v", err)
		respondErr(w, 422, "invalid request body")
		return
	}

	if eventReq.Event != "user.upgraded" {
		w.WriteHeader(204)
		return
	}

	userUUID, err := uuid.Parse(eventReq.Data.UserID)
	if err != nil {
		log.Printf("Failed to parse UUID: %v", err)
		respondErr(w, 422, "invalid UUID provided")
		return
	}

	_, err = cfg.dbQueries.UpdateUserStatus(r.Context(), userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("Requested user not found in the db: %v", err)
			respondErr(w, 404, "user not found")
		}
		log.Printf("db failed to update user status: %v", err)
		respondErr(w, 500, "Something went wrong on the db")
		return
	}

	w.WriteHeader(204)
}

/* ***************************************************************************
*	Response Helper                                                            *
* ***************************************************************************/

func decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var ret T
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&ret); err != nil {
		log.Printf("Error decoding: %v", err)
		respondErr(w, 400, "Invalid Request Payload")
		return ret, err
	}
	return ret, nil
}

func respond(w http.ResponseWriter, code int, payload any) {
	if err := respondWithJSON(w, code, payload); err != nil {
		log.Printf("Error responding: %s", err)
	}
}

func respondErr(w http.ResponseWriter, code int, msg string) {
	if err := respondWithError(w, code, msg); err != nil {
		log.Printf("Error responding: %s", err)
	}
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)

	return json.NewEncoder(w).Encode(payload)
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
	return respondWithJSON(w, code, map[string]string{"error": msg})
}

/* ***************************************************************************
* ***************************************************************************/

func main() {
	const port = "8080"
	const filepathRoot = "."
	if err := godotenv.Load(); err != nil {
		log.Printf("Failed to load .env: %v", err)
		return
	}

	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)
	if err != nil {
		return
	}

	cfg := &apiConfig{}
	cfg.dbQueries = dbQueries
	cfg.platform = os.Getenv("PLATFORM")
	cfg.secret = os.Getenv("SECRET")
	cfg.polkaKey = os.Getenv("POLKA_KEY")
	mux := http.NewServeMux()
	server := http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	fileHandler := http.StripPrefix("/app", http.FileServer(http.Dir(filepathRoot)))
	mux.HandleFunc("GET /admin/metrics", cfg.handlerWriteMetrics)
	mux.HandleFunc("POST /admin/reset", cfg.handlerReset)

	mux.HandleFunc("GET /api/chirps", cfg.handlerGetChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", cfg.handlerGetChirp)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", cfg.handlerDeleteChirp)
	mux.HandleFunc("POST /api/chirps", cfg.handlerCreateChirp)

	mux.HandleFunc("POST /api/users", cfg.handlerUserCreate)
	mux.HandleFunc("POST /api/login", cfg.handlerLogin)
	mux.HandleFunc("POST /api/refresh", cfg.handlerRefreshToken)
	mux.HandleFunc("POST /api/revoke", cfg.handlerRevokeToken)
	mux.HandleFunc("PUT /api/users", cfg.handlerUpdateUsers)

	mux.HandleFunc("POST /api/polka/webhooks", cfg.handlerPolkaWebhook)

	mux.HandleFunc("GET /api/healthz", handlerHealthz)
	mux.Handle("/app/", cfg.middlewareMetricsInc(fileHandler))

	log.Fatal(server.ListenAndServe())
}
