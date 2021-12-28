package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"database/sql"

	"github.com/fox-one/mixin-sdk-go"
	"github.com/gofrs/uuid"
)

var (
	client *mixin.Client
	config = flag.String("config", "", "keystore file path")
)

func main() {
	flag.Parse()

	f, err := os.Open(*config)
	if err != nil {
		log.Panicln(err)
	}

	var store mixin.Keystore
	if err := json.NewDecoder(f).Decode(&store); err != nil {
		log.Panicln(err)
	}

	client, err = mixin.NewFromKeystore(&store)
	if err != nil {
		log.Panicln(err)
	}

	db, err := sql.Open("mysql", "sticker:sticker@/sticker")
	if err != nil {
		log.Fatal(err)
	}

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}

	h := func(ctx context.Context, msg *mixin.MessageView, userID string) error {
		if userID, _ := uuid.FromString(msg.UserID); userID == uuid.Nil {
			return nil
		}

		return handleMessage(ctx, db, msg)
	}

	ctx := context.Background()

	for {
		if err := client.LoopBlaze(ctx, mixin.BlazeListenFunc(h)); err != nil {
			log.Printf("LoopBlaze: %v", err)
		}

		time.Sleep(time.Second)
	}
}
