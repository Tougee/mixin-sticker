package main

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type Sticker struct {
	AlbumID     string `json:"album_id"`
	StickerName string `json:"sticker_name"`
	AlbumName   string `json:"album_name"`
	Url         string `json:"url"`
}

func findByAlbumName(db *sql.DB, albumName string) ([]Sticker, error) {
	results, err := db.Query("SELECT * FROM sticker WHERE album_name = ?", albumName)
	if err != nil {
		return nil, err
	}
	defer results.Close()
	var stickers []Sticker
	for results.Next() {
		var sticker Sticker
		results.Scan(&sticker.AlbumID, &sticker.StickerName, &sticker.AlbumName, &sticker.Url)
		stickers = append(stickers, sticker)
	}
	return stickers, nil
}
