package main

import (
	"log"
	"os"

	"github.com/nejstastnejsistene/frotz-slack-bot/rtm"
)

func onMessage(msg rtm.Message, respond chan rtm.Message) {
	if !directMessage(msg) {
		return
	}
	user := msg["user"].(string)
	text := msg["text"].(string)
	log.Printf("%s: \"%s\"\n", user, text)
	respond <- rtm.Message{
		"type":    msg["type"],
		"channel": msg["channel"],
		"text":    msg["text"],
	}
}

func directMessage(m rtm.Message) bool {
	// Ignore reply_to messages; these are for already sent messages.
	if _, ok := m["reply_to"]; ok {
		return false
	}
	msgType, ok := m["type"].(string)
	if !ok {
		return false
	}
	channel, ok := m["channel"].(string)
	if !ok {
		return false
	}
	if _, ok := m["user"].(string); !ok {
		return false
	}
	if _, ok := m["text"].(string); !ok {
		return false
	}
	return msgType == "message" && len(channel) > 0 && channel[0] == 'D'
}

func main() {
	var token string
	if token = os.Getenv("TOKEN"); token == "" {
		log.Fatal("TOKEN not specified")
	}
	rtm.LoopForever(token, onMessage)
}
