package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/fox-one/mixin-sdk-go"
)

func addSticker(localUrl string) (*MixinSticker, error) {
	log.Printf("addSticker: %s", localUrl)
	data, err := ioutil.ReadFile(localUrl)
	if err != nil {
		return nil, err
	}

	paras := map[string]interface{}{
		"data_base64": base64.RawURLEncoding.EncodeToString(data),
	}
	var mixinSticker MixinSticker
	if err = client.Post(ctx, "/stickers/favorite/add", paras, &mixinSticker); err != nil {
		switch {
		case mixin.IsErrorCodes(err, TooManyStickers):
			log.Println("Too many stickers")
			clearPersonalStickers()
		case mixin.IsErrorCodes(err, BadData):
			clearPersonalStickers()
			return nil, fmt.Errorf("bad data")
		default:
		}
		return nil, err
	}

	return &mixinSticker, nil
}

func checkPersonalStickers() (int, error) {
	var stickers []MixinSticker
	if err := client.Get(ctx, fmt.Sprintf("/stickers/albums/%s", BotPersonalAlbumID), nil, &stickers); err != nil {
		return -1, err
	}
	return len(stickers), nil
}

func clearPersonalStickers() error {
	var albums []MixinAlbum
	if err := client.Get(ctx, "/stickers/albums", nil, &albums); err != nil {
		return err
	}
	var albumId string
	for _, album := range albums {
		if album.Category == "PERSONAL" {
			albumId = album.AlbumID
		}
	}

	var stickers []MixinSticker
	if err := client.Get(ctx, fmt.Sprintf("/stickers/albums/%s", albumId), nil, &stickers); err != nil {
		return err
	}
	var ids []string
	for _, sticker := range stickers {
		ids = append(ids, sticker.StickerID)
	}
	return removeStickers(ids...)
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

type MixinAlbum struct {
	AlbumID     string    `json:"album_id"`
	Name        string    `json:"name"`
	IconUrl     string    `json:"icon_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	UserID      string    `json:"user_id"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
}
