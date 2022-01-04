package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/fox-one/mixin-sdk-go"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/rs/cors"
)

var (
	accessToken string
	loading     bool
)

const (
	TooManyStickers = 20126

	BotPersonalAlbumID = "1a472cbb-3c55-497a-bec4-d8be0d9af502"
	AdminUserID        = "d3bee23a-81d4-462e-902a-22dae9ef89ff"
)

func StartHttpServer() {
	{
		mux := chi.NewMux()
		mux.Use(middleware.Recoverer)
		mux.Use(middleware.StripSlashes)
		mux.Use(cors.AllowAll().Handler)
		mux.Use(middleware.Logger)

		mux.Get("/hc", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		})

		mux.Get("/", renderIndexPage)
		mux.Handle("/oauth", HandleOauth(client.ClientID, *clientSecret))

		mux.Mount("/api", HandleRest())

		go http.ListenAndServe(":8080", mux)
	}
}

func renderIndexPage(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("index.html")
	if err != nil {
		renderError(w, err, 500)
		return
	}
	type IndexPageParams struct {
		Signed   bool
		Loading  bool
		ClientID string
	}
	t.Execute(w, IndexPageParams{
		Signed:   accessToken != "",
		Loading:  loading,
		ClientID: client.ClientID,
	})
}

func HandleOauth(clientID, clientSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var scope string
		var err error

		log.Printf("clientSecret: %s", clientSecret)
		accessToken, scope, err = mixin.AuthorizeToken(ctx, clientID, clientSecret, r.URL.Query().Get("code"), "")
		if err != nil {
			log.Println("AuthorizeToken:", err)
			renderError(w, err, 401)
			return
		}
		log.Println("accessToken:", accessToken)

		if !strings.Contains(scope, "PROFILE:READ") {
			renderError(w, fmt.Errorf("Incorrect scope"), 400)
			return
		}

		user, err := mixin.UserMe(ctx, accessToken)
		if err != nil {
			renderError(w, err, 500)
			return
		}

		// You may wanna save the user and access token to database
		log.Println(user, accessToken)

		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	}
}

func HandleRest() http.Handler {
	r := chi.NewRouter()

	r.Get("/me", getMe)

	return r
}

func getMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	me, err := mixin.UserMe(ctx, accessToken)
	if err != nil {
		renderError(w, err, 500)
	}
	renderJSON(w, me)
}

func renderJSON(w http.ResponseWriter, object interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(object); err != nil {
		renderError(w, fmt.Errorf("Unknown error"), 500)
	}
}

func renderError(w http.ResponseWriter, err error, code int) {
	http.Error(w, err.Error(), code)
}
