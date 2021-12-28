package main

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type Sticker struct {
	StickerName    string `json:"sticker_name"`
	AlbumID        string `json:"album_id"`
	AlbumName      string `json:"album_name"`
	Url            string `json:"url"`
	MixinStickerID string `json:"mixin_sticker_id"`
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
		results.Scan(&sticker.StickerName, &sticker.AlbumID, &sticker.AlbumName, &sticker.Url, &sticker.MixinStickerID)
		stickers = append(stickers, sticker)
	}
	return stickers, nil
}

func updateMixinStickerID(db *sql.DB, sticker Sticker, mixinStickerID string) (bool, error) {
	result, err := db.Exec("UPDATE sticker SET mixin_sticker_id = ? WHERE album_id = ? AND sticker_name = ?", mixinStickerID, sticker.AlbumID, sticker.StickerName)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}
