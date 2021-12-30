package main

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type Sticker struct {
	StickerID      string `json:"sticker_id"`
	Url            string `json:"url"`
	StickerName    string `json:"sticker_name"`
	AlbumID        string `json:"album_id"`
	AlbumName      string `json:"album_name"`
	LocalUrl       string `json:"local_url"`
	MixinStickerID string `json:"mixin_sticker_id"`
}

func findByAlbumName(db *sql.DB, albumName string) ([]Sticker, error) {
	results, err := db.Query("SELECT * FROM sticker WHERE album_name = ? ORDER BY album_name", albumName)
	if err != nil {
		return nil, err
	}
	defer results.Close()
	var stickers []Sticker
	for results.Next() {
		var sticker Sticker
		results.Scan(&sticker.StickerID, &sticker.Url, &sticker.StickerName, &sticker.AlbumID, &sticker.AlbumName, &sticker.LocalUrl, &sticker.MixinStickerID)
		stickers = append(stickers, sticker)
	}
	return stickers, nil
}

func findByUrl(db *sql.DB, url string) (*Sticker, error) {
	sticker := Sticker{}
	row, err := db.Query("SELECT sticker_id, url, local_url, mixin_sticker_id FROM sticker WHERE url = ?", url)
	defer row.Close()
	if err != nil {
		return nil, err
	}
	if !row.Next() {
		return nil, nil
	}
	row.Scan(&sticker.StickerID, &sticker.Url, &sticker.LocalUrl, &sticker.MixinStickerID)
	return &sticker, err
}

func updateMixinStickerID(db *sql.DB, stickerID string, mixinStickerID string) (bool, error) {
	result, err := db.Exec("UPDATE sticker SET mixin_sticker_id = ? WHERE sticker_id = ?", mixinStickerID, stickerID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}
