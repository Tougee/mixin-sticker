package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/fox-one/mixin-sdk-go"
	"github.com/gofrs/uuid"
)

func handleMessage(msg *mixin.MessageView) error {
	log.Printf("handle message ID: %s", msg.MessageID)
	if msg.Category != mixin.MessageCategoryPlainText {
		data := fmt.Sprintf("Only support message like https://t.me/addstickers/stpcts")
		return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(data))
	}

	msgContent, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(string(msgContent), "https://t.me/addstickers/") {
		data := fmt.Sprintf("Only support message like https://t.me/addstickers/stpcts")
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
			log.Printf("Failed to start cmd: %v", err)
			return nil
		}

		if err := cmd.Wait(); err != nil {
			log.Printf("Cmd returned error: %v", err)
		}

		fmt.Println("spider done")
		stickers, err = findByAlbumName(db, albumName)
		if err != nil {
			return err
		}
	}

	fmt.Println("stickers len:", len(stickers))
	if stickers != nil && len(stickers) > 0 {
		return respondSticker(stickers, msg)
	}

	return nil
}

func respondSticker(stickers []Sticker, msg *mixin.MessageView) error {
	var replies []*mixin.MessageRequest
	for _, sticker := range stickers {
		log.Printf("sticker: %v", sticker)
		mixinStickerID := sticker.MixinStickerID
		if mixinStickerID == "" {
			mixinSticker, err := addSticker(sticker)
			if err != nil {
				log.Printf("addSticker error: %v", err)
				continue
			}
			success, err := updateMixinStickerID(db, sticker, mixinSticker.StickerID)
			if err != nil || !success {
				log.Printf("updateMixinStickerID error: %v, success: %v", err, success)
				continue
			}
			mixinStickerID = mixinSticker.StickerID
		}

		json, err := json.Marshal(map[string]string{
			"sticker_id": mixinStickerID,
		})
		if err != nil {
			log.Printf("json marshal error: %v", err)
			continue
		}

		payload := base64.StdEncoding.EncodeToString(json)
		id, _ := uuid.FromString(msg.MessageID)
		newMessageID := uuid.NewV5(id, "reply"+mixinStickerID).String()
		reply := &mixin.MessageRequest{
			ConversationID: msg.ConversationID,
			RecipientID:    msg.UserID,
			MessageID:      newMessageID,
			Category:       mixin.MessageCategoryPlainSticker,
			Data:           payload,
		}
		replies = append(replies, reply)
	}

	log.Printf("replies len: %v", len(replies))
	if len(replies) == 0 {
		return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte("No sticker found, please make sure the link is valid, or you can contact developer."))
	}
	return client.SendMessages(ctx, replies)
}

func respond(ctx context.Context, msg *mixin.MessageView, category string, data []byte) error {
	id, _ := uuid.FromString(msg.MessageID)
	newMessageID := uuid.NewV5(id, "reply").String()
	return sendMessage(ctx, newMessageID, msg.ConversationID, msg.UserID, category, data)
}

func respondError(ctx context.Context, msg *mixin.MessageView, err error) error {
	errString := fmt.Sprintln(err)
	respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(errString))
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

func sendMessages(ctx context.Context, messageID, conversationID, recipientID, category string, data []byte) error {
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
