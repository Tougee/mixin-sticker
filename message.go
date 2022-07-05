package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fox-one/mixin-sdk-go"
	"github.com/gofrs/uuid"
)

var supported_hint = `Send me link like:
	1. Telegram sticker album share link, e.g., https://t.me/addstickers/stpcts
	2. Telegram web link, e.g., https://tlgrm.eu/stickers/stpcts
`

func handleMessage(msg *mixin.MessageView) error {
	if msg.Category == mixin.MessageCategoryPlainData {
		data, err := base64.StdEncoding.DecodeString(msg.Data)
		if err != nil {
			return err
		}
		dataMessage := mixin.DataMessage{}
		err = json.Unmarshal(data, &dataMessage)
		if err != nil {
			return err
		}
		log.Printf("dataMessage %v", dataMessage)
		if strings.HasSuffix(dataMessage.Name, "tgs") {
			err = handleTgs(&dataMessage, msg)
		} else if strings.HasSuffix(dataMessage.Name, "zip") {
			respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(fmt.Sprintf("try analysis %s, please wait...", dataMessage.Name)))
			go func(d *mixin.DataMessage, cid, mid, uid string) {
				handleTgsZip(d, cid, mid, uid)
			}(&dataMessage, msg.ConversationID, msg.MessageID, msg.UserID)
		}
		if err != nil {
			respondErrorMsg := fmt.Sprintf("add sticker error: %v", err)
			log.Println(respondErrorMsg)
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
		}
		return nil
	} else if msg.Category != mixin.MessageCategoryPlainText {
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

	tgAlbumLink := strings.HasPrefix(content, "https://t.me/addstickers/") || strings.HasPrefix(content, "https://tlgrm.eu/stickers/")
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

func handleTgsZip(data *mixin.DataMessage, cid, mid, uid string) error {
	fileName := strings.TrimSuffix(data.Name, filepath.Ext(data.Name))
	zipFile, err := downloadAttachment(data, fmt.Sprintf("/tmp/%s.zip", fileName))
	if err != nil {
		return err
	}
	defer os.Remove(zipFile.Name())

	files, err := Unzip(zipFile.Name(), "tmp/outdir")
	if err != nil {
		return err
	}
	defer os.RemoveAll("tmp/outdir")

	var replies []*mixin.MessageRequest
	var clearIds []string
	for _, fn := range files {
		if !strings.HasSuffix(fn, "tgs") {
			continue
		}

		f, err := os.Open(fn)
		if err != nil {
			log.Println(err)
			continue
		}

		mixinSticker, json, err := handleTgsFile(f, fn)
		if err != nil {
			return err
		}

		clearIds = append(clearIds, mixinSticker.StickerID)

		payload := base64.StdEncoding.EncodeToString(json)
		id, _ := uuid.FromString(mid)
		newMessageID := uuid.NewV5(id, "reply"+mixinSticker.StickerID).String()
		reply := &mixin.MessageRequest{
			ConversationID: cid,
			RecipientID:    uid,
			MessageID:      newMessageID,
			Category:       mixin.MessageCategoryPlainSticker,
			Data:           payload,
		}
		replies = append(replies, reply)
	}

	err = removeStickers(clearIds...)
	if err != nil {
		log.Printf("removeStickers error: %v", err)
	}

	log.Printf("replies len: %v", len(replies))
	if len(replies) == 0 {
		return nil
	}

	chunkSize := 10
	for i := 0; i < len(replies); i += chunkSize {
		end := i + chunkSize
		if end > len(replies) {
			end = len(replies)
		}
		err = client.SendMessages(ctx, replies[i:end])
		if err != nil {
			log.Println(err)
		}
	}
	return nil
}

func handleTgs(data *mixin.DataMessage, msg *mixin.MessageView) error {
	fileName := strings.TrimSuffix(data.Name, filepath.Ext(data.Name))
	file, err := downloadAttachment(data, fmt.Sprintf("tmp/%s.gzip", fileName))
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())

	mixinSticker, json, err := handleTgsFile(file, fileName)
	if err != nil {
		return err
	}

	payload := base64.StdEncoding.EncodeToString(json)
	id, _ := uuid.FromString(msg.MessageID)
	newMessageID := uuid.NewV5(id, "reply"+mixinSticker.StickerID).String()
	reply := &mixin.MessageRequest{
		ConversationID: msg.ConversationID,
		RecipientID:    msg.UserID,
		MessageID:      newMessageID,
		Category:       mixin.MessageCategoryPlainSticker,
		Data:           payload,
	}
	return client.SendMessage(ctx, reply)
}

func downloadAttachment(data *mixin.DataMessage, fileName string) (*os.File, error) {
	attachment, err := client.ShowAttachment(ctx, data.AttachmentID)
	if err != nil {
		return nil, err
	}

	file, err := os.Create(fileName)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(attachment.ViewURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return nil, err
	}

	return file, err
}

func handleTgsFile(file *os.File, fileName string) (*MixinSticker, []byte, error) {
	unzipFile, err := os.Create(fmt.Sprintf("%s.json", fileName))
	if err != nil {
		return nil, nil, err
	}
	defer os.Remove(unzipFile.Name())

	fileBytes, err := os.ReadFile(file.Name())
	if err != nil {
		return nil, nil, err
	}

	unzipBytes, err := UngzipData(fileBytes)
	if err != nil {
		return nil, nil, err
	}

	err = os.WriteFile(unzipFile.Name(), unzipBytes, 0644)
	if err != nil {
		return nil, nil, err
	}

	mixinSticker, err := addSticker(unzipFile.Name())
	if err != nil {
		return nil, nil, err
	}

	json, err := json.Marshal(map[string]string{
		"sticker_id": mixinSticker.StickerID,
	})
	if err != nil {
		return nil, nil, err
	}

	return mixinSticker, json, nil
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

		cmdStr := fmt.Sprintf("/usr/bin/python3 spider.py --url=%v", content)
		err = callSpider(cmdStr)
		if err != nil {
			respondErrorMsg = "Failed to fetch lottie file, please try again later."
			log.Println(respondErrorMsg)
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
		}

		fmt.Println("spider done")
		sticker, err = findByUrl(db, content)
		if err != nil || sticker == nil {
			respondErrorMsg = "Valid lottie file not found, please try again later or contact developer."
			log.Println(respondErrorMsg)
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
		}
	}

	mixinStickerID := sticker.MixinStickerID
	if mixinStickerID == "" {
		mixinSticker, err := addSticker(sticker.LocalUrl)
		if err != nil {
			respondErrorMsg = fmt.Sprintf("addSticker error: %v", err)
			log.Println(respondErrorMsg)
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
		}
		success, err := updateMixinStickerID(db, sticker.StickerID, mixinSticker.StickerID)
		if err != nil || !success {
			respondErrorMsg = fmt.Sprintf("updateMixinStickerID error: %v, success: %v", err, success)
			log.Println(respondErrorMsg)
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
		respondErrorMsg = fmt.Sprintf("json marshal error: %v", err)
		log.Println(respondErrorMsg)
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
	var albumName string
	if strings.HasPrefix(content, "https://t.me/addstickers/") {
		albumName = strings.TrimPrefix(content, "https://t.me/addstickers/")
	} else {
		albumName = strings.TrimPrefix(content, "https://tlgrm.eu/stickers/")
	}
	stickers, err := findByAlbumName(db, albumName)
	var respondErrorMsg string
	if err != nil {
		log.Printf("findByAlbumName error: %v", err)
	}

	if len(stickers) == 0 {
		respond(ctx, msg, mixin.MessageCategoryPlainText, []byte("No cache founded, fetching from Telegram, please wait..."))

		cmdStr := fmt.Sprintf("/usr/bin/python3 spider.py --album=%v", albumName)
		err = callSpider(cmdStr)
		if err != nil {
			respondErrorMsg = "Failed to fetch from Telegram, please try again later."
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
		}

		fmt.Println("spider done")
		stickers, err = findByAlbumName(db, albumName)
		if err != nil {
			respondErrorMsg = "Valid album not found, please try again later or contact developer."
			return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
		}
	}

	fmt.Println("tg album stickers len:", len(stickers))
	if len(stickers) > 0 {
		return respondSticker(stickers, msg)
	} else {
		respondErrorMsg = "Valid stickers not found from Telegram, please try again later or contact developer."
		return respond(ctx, msg, mixin.MessageCategoryPlainText, []byte(respondErrorMsg))
	}
}

func respondSticker(stickers []Sticker, msg *mixin.MessageView) error {
	var replies []*mixin.MessageRequest
	var clearIds []string
	for _, sticker := range stickers {
		mixinStickerID := sticker.MixinStickerID
		if mixinStickerID == "" {
			mixinSticker, err := addSticker(sticker.LocalUrl)
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

	chunkSize := 10
	for i := 0; i < len(replies); i += chunkSize {
		end := i + chunkSize
		if end > len(replies) {
			end = len(replies)
		}
		client.SendMessages(ctx, replies[i:end])
	}
	return nil
}

func respond(ctx context.Context, msg *mixin.MessageView, category string, data []byte) error {
	id, _ := uuid.FromString(msg.MessageID)
	newMessageID := uuid.NewV5(id, fmt.Sprintf("reply %v", rand.Intn(100000))).String()
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
