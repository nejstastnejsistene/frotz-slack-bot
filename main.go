package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/nejstastnejsistene/frotz-slack-bot/rtm"
)

var (
	games = make(map[string]*Zork)
	mutex sync.RWMutex
)

func onMessage(msg rtm.Message, respond chan rtm.Message) {
	if !directMessage(msg) {
		return
	}
	user := msg["user"].(string)
	text := msg["text"].(string)

	var response string
	defer func() {
		respond <- rtm.NewResponse(msg, "```"+response+"```")
	}()

	mutex.RLock()
	z := games[user]
	mutex.RUnlock()

	if z == nil {
		z, output, err := StartZork("/home/peter/code/frotz/dfrotz", "/home/peter/code/zork-ai/ZORK1.DAT")
		if err != nil {
			response = fmt.Sprint("[error: %s]", err)
		} else {

			mutex.Lock()
			games[user] = z
			mutex.Unlock()

			response = output
		}
	} else {
		output, err := z.ExecuteCommand(text)
		if err != nil {
			mutex.Lock()
			delete(games, user)
			mutex.Unlock()

			response = fmt.Sprint("[error: %s]", err)
			if err == CleanExit {
				response = "[process exitted cleanly]"
			}
		} else {
			response = output
		}
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
