package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fox-one/mixin-sdk-go"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/rs/cors"
)

var (
	accessToken string
	loading     bool
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

		go http.ListenAndServe(":8080", mux)
	}
}

func renderIndexPage(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("index.html")
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

func addSticker(sticker Sticker) (*MixinSticker, error) {
	log.Printf("addSticker: %+v", sticker)
	data, err := ioutil.ReadFile(sticker.Url)
	if err != nil {
		return nil, err
	}

	paras := map[string]interface{}{
		"data_base64": base64.RawURLEncoding.EncodeToString(data),
	}
	var mixinSticker MixinSticker
	if err = client.Post(ctx, "/stickers/favorite/add", paras, &mixinSticker); err != nil {
		return nil, err
	}

	return &mixinSticker, nil
}

func removeStickers(ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	reqBody, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	if err = client.Post(ctx, "/stickers/favorite/remove", reqBody, nil); err != nil {
		return err
	}
	return nil
}

func HandleOauth(clientID, clientSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// because we set 'http://localhost:8080/oauth' as the callback url at Developer Dashboard
		// Mixin's OAuth will redirect each successful OAuth request to the callback url
		// with a `code`, which i will use it to exchange for the access token.
		ctx := r.Context()
		var scope string
		var err error

		accessToken, scope, err = mixin.AuthorizeToken(ctx, clientID, clientSecret, r.URL.Query().Get("code"), "")
		if err != nil {
			renderError(w, err, 401)
			return
		}

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

func renderJSON(w http.ResponseWriter, object interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(object); err != nil {
		renderError(w, fmt.Errorf("Unknown error"), 500)
	}
}

func renderError(w http.ResponseWriter, err error, code int) {
	http.Error(w, err.Error(), code)
}

type StickerRequest struct {
	DataBase64 string `json:"data_base64"`
}

type MixinSticker struct {
	StickerID   string    `json:"sticker_id"`
	AssetUrl    string    `json:"asset_url"`
	AssetType   string    `json:"asset_type"`
	AssetWidth  int       `json:"asset_width"`
	AssetHeight int       `json:"asset_height"`
	CreatedAt   time.Time `json:"created_at"`
}