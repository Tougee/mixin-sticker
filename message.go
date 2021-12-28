package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/fox-one/mixin-sdk-go"
	"github.com/gofrs/uuid"
)

func handleMessage(ctx context.Context, db *sql.DB, msg *mixin.MessageView) error {
	if msg.Category != mixin.MessageCategoryPlainText {
		data := fmt.Sprintf("Only support message like 'https://t.me/addstickers/stpcts'")
		return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(data))
	}

	msgContent, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(string(msgContent), "https://t.me/addstickers/") {
		data := fmt.Sprintf("Only support message like 'https://t.me/addstickers/stpcts'")
		return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(data))
	}

	albumName := strings.TrimPrefix(string(msgContent), "https://t.me/addstickers/")
	stickers, err := findByAlbumName(db, albumName)
	if err != nil {
		return err
	}

	if stickers == nil || len(stickers) == 0 {
		respond(ctx, msg, mixin.MessageCategoryPlainText, []byte("No cache founded, fetching from telegram, please wait..."))

		cmdStr := fmt.Sprintf("python3 spider.py --album=%v", albumName)
		fmt.Println("no stickers cmdStr:", cmdStr)
		cmd := exec.Command("bash", "-c", cmdStr)
		if err := cmd.Start(); err != nil {
			fmt.Printf("Failed to start cmd: %v", err)
			return nil
		}

		if err := cmd.Wait(); err != nil {
			fmt.Printf("Cmd returned error: %v", err)
		}

		fmt.Println("spider done")
		stickers, err = findByAlbumName(db, albumName)
		if err != nil {
			return err
		}
	}

	fmt.Println("stickers:", stickers)
	// if stickers != nil && len(stickers) > 0 {
	// 	var url string
	// 	for _, sticker := range stickers {
	// 		url += sticker.Url + "\n"
	// 	}
	// 	fmt.Printf("url: %v", url)
	// 	return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(url))
	// }

	// file, _ := ioutil.ReadFile(stickers[0].Url)
	mixinSticker := MixinSticker{
		StickerID: "14d15a07-d028-4c4a-ac1c-6a22117c5666",
	}
	json, _ := json.Marshal(mixinSticker)
	return respond(ctx, msg, mixin.MessageCategoryPlainSticker, json)

	// return nil
}

type MixinSticker struct {
	StickerID string `json:"sticker_id"`
}

func respond(ctx context.Context, msg *mixin.MessageView, category string, data []byte) error {
	id, _ := uuid.FromString(msg.MessageID)
	newMessageID := uuid.NewV5(id, "reply"+string(data[:5])).String()
	return sendMessage(ctx, newMessageID, msg.ConversationID, msg.UserID, category, data)
}

func respondError(ctx context.Context, msg *mixin.MessageView, err error) error {
	respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(fmt.Sprintln(err)))
	return nil
}

func sendMessage(ctx context.Context, messageID, conversationID, recipientID, category string, data []byte) error {
	payload := base64.StdEncoding.EncodeToString(data)
	reply := &mixin.MessageRequest{
		ConversationID: conversationID,
		RecipientID:    recipientID,
		MessageID:      messageID,
		Category:       category,
		Data:           payload,
	}
	return client.SendMessage(ctx, reply)
}
