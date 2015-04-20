package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/websocket"
)

var token string

func init() {
	if token = os.Getenv("TOKEN"); token == "" {
		log.Fatal("TOKEN not specified")
	}
}

//{"type":"message","channel":"D04FLBCPZ","user":"U02D58RR5","text":"howdy ho","ts":"1429561458.000015","team":"T02D58RR3"}
type Message struct {
	Type    string
	Channel string
	User    string
	Text    string
	Ts      string
	Team    string
}

func webSocketUrl() (url string, err error) {
	resp, err := http.Get("https://slack.com/api/rtm.start?token=" + token)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var body map[string]interface{}
	if err = json.Unmarshal(buf, &body); err != nil {
		return
	}
	url, ok := body["url"].(string)
	if !ok {
		err = errors.New("unable to parse rtm url")
	}
	return
}

func main() {
	url, err := webSocketUrl()
	if err != nil {
		log.Fatal(err)
	}
	origin := "http://localhost/"
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	input := make(chan []byte)
	go func() {
		for {
			var buf []byte
			if err := websocket.Message.Receive(ws, &buf); err != nil {
				log.Println(ws)
				log.Fatal(err)
			}
			if len(buf) > 0 {
				input <- buf
			}
		}
	}()

	output := make(chan map[string]interface{})
	go func() {
		id := 0
		for {
			msg := <-output
			msg["id"] = id
			buf, _ := json.MarshalIndent(msg, "", "  ")
			log.Println("sending message")
			fmt.Println(string(buf))
			if err := websocket.JSON.Send(ws, msg); err != nil {
				log.Fatal(err)
			}
			id++
		}
	}()

	messages := make(chan Message)
	go func() {
		for {
			select {
			case buf := <-input:
				var msg Message
				if err := json.Unmarshal(buf, &msg); err != nil {
					log.Fatal("unable to parse rtm message")
				}
				switch msg.Type {
				case "message":
					if len(msg.Channel) > 0 && msg.Channel[0] == 'D' {
						messages <- msg
					}
				default:
					log.Println("ignoring", msg.Type)
				}
			case <-time.After(5 * time.Second):
				log.Println("timeout, pinging")
				output <- map[string]interface{}{"type": "ping"}
			}
		}
	}()
	for m := range messages {
		fmt.Println(m)
		output <- map[string]interface{}{
			"type":    m.Type,
			"channel": m.Channel,
			"text":    m.Text,
		}
	}
}
