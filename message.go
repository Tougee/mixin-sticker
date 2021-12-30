package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"

	"github.com/fox-one/mixin-sdk-go"
	"github.com/gofrs/uuid"
)

var supported_hint = `Currently supported types:
	1. Telegram sticker album link, e.g., https://t.me/addstickers/stpcts
	2. Directly lottie file link, e.g., https://assets9.lottiefiles.com/packages/lf20_muiaursk.json
`

func handleMessage(msg *mixin.MessageView) error {
	if msg.Category != mixin.MessageCategoryPlainText {
		return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(supported_hint))
	}

	msgContent, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		return err
	}

	content := string(msgContent)

	if content == "clear-albums" && msg.UserID == AdminUserID {
		err = clearPersonalStickers()
		if err != nil {
			return respondError(ctx, msg, err)
		} else {
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte("clear success"))
		}
	} else if content == "check-albums" && msg.UserID == AdminUserID {
		count, err := checkPersonalStickers()
		if err != nil {
			return respondError(ctx, msg, err)
		} else {
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(fmt.Sprintf("Current personal sticker count %v", count)))
		}
	}

	tgAlbumLink := strings.HasPrefix(content, "https://t.me/addstickers/")
	lottieLink := strings.HasSuffix(content, ".json")
	if !tgAlbumLink && !lottieLink {
		return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(supported_hint))
	}

	if tgAlbumLink {
		return handleTgAlbum(content, msg)
	} else if lottieLink {
		return handleLottie(content, msg)
	}

	return nil
}

func handleLottie(content string, msg *mixin.MessageView) error {
	sticker, err := findByUrl(db, content)
	if err != nil {
		log.Printf("findByUrl error: %v", err)
	}

	var respondErrorMsg string
	if sticker == nil {
		respondErrorMsg = fmt.Sprintf("No cache founded, fetching from %v, please wait...", content)
		respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))

		cmdStr := fmt.Sprintf("python3 spider.py --url=%v", content)
		err = callSpider(cmdStr)
		if err != nil {
			respondErrorMsg = "Failed to fetch lottie file, please try again later."
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
		}

		fmt.Println("spider done")
		sticker, err = findByUrl(db, content)
		if err != nil || sticker == nil {
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
		}
	}

	mixinStickerID := sticker.MixinStickerID
	if mixinStickerID == "" {
		mixinSticker, err := addSticker(*sticker)
		if err != nil {
			log.Printf("addSticker error: %v", err)
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
		}
		success, err := updateMixinStickerID(db, sticker.StickerID, mixinSticker.StickerID)
		if err != nil || !success {
			log.Printf("updateMixinStickerID error: %v, success: %v", err, success)
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
		}
		mixinStickerID = mixinSticker.StickerID
	}
	if mixinStickerID == "" {
		return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
	}

	err = removeStickers(mixinStickerID)
	if err != nil {
		log.Printf("removeStickers error: %v", err)
	}

	json, err := json.Marshal(map[string]string{
		"sticker_id": mixinStickerID,
	})
	if err != nil {
		log.Printf("json marshal error: %v", err)
		return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
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
	return client.SendMessage(ctx, reply)
}

func handleTgAlbum(content string, msg *mixin.MessageView) error {
	albumName := strings.TrimPrefix(content, "https://t.me/addstickers/")
	stickers, err := findByAlbumName(db, albumName)
	if err != nil {
		return err
	}

	if stickers == nil || len(stickers) == 0 {
		respond(ctx, msg, mixin.MessageCategoryPlainText, []byte("No cache founded, fetching from telegram, please wait..."))

		cmdStr := fmt.Sprintf("python3 spider.py --album=%v", albumName)
		err = callSpider(cmdStr)
		if err != nil {
			return err
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
	var clearIds []string
	for _, sticker := range stickers {
		log.Printf("sticker: %v", sticker)
		mixinStickerID := sticker.MixinStickerID
		if mixinStickerID == "" {
			mixinSticker, err := addSticker(sticker)
			if err != nil {
				log.Printf("addSticker error: %v", err)
				continue
			}
			success, err := updateMixinStickerID(db, sticker.StickerID, mixinSticker.StickerID)
			if err != nil || !success {
				log.Printf("updateMixinStickerID error: %v, success: %v", err, success)
				continue
			}
			mixinStickerID = mixinSticker.StickerID
		}

		clearIds = append(clearIds, mixinStickerID)

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

	err := removeStickers(clearIds...)
	if err != nil {
		log.Printf("removeStickers error: %v", err)
	}

	log.Printf("replies len: %v", len(replies))
	if len(replies) == 0 {
		return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte("No sticker found, please make sure the link is valid, or you can contact developer."))
	}
	return client.SendMessages(ctx, replies)
}

func respond(ctx context.Context, msg *mixin.MessageView, category string, data []byte) error {
	id, _ := uuid.FromString(msg.MessageID)
	newMessageID := uuid.NewV5(id, fmt.Sprintf("reply %v", rand.Intn(100))).String()
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
