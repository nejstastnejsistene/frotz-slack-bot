package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/websocket"
)

type Message map[string]interface{}

func (m Message) Type() string    { return m["type"].(string) }
func (m Message) Channel() string { return m["channel"].(string) }
func (m Message) User() string    { return m["user"].(string) }
func (m Message) Text() string    { return m["text"].(string) }

func (m Message) DirectMessage() bool {
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

func (m Message) String() string {
	buf, _ := json.Marshal(m)
	return string(buf)
}

var token string

func init() {
	if token = os.Getenv("TOKEN"); token == "" {
		log.Fatal("TOKEN not specified")
	}
}

func webSocketUrl() (url string, err error) {
	log.Println("authenticating with rtm")
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

func connectToRtm() (ws *websocket.Conn, input, output chan Message, err error) {
	var url string
	url, err = webSocketUrl()
	if err != nil {
		return
	}

	log.Println("connecting to rtm")
	origin := "http://localhost/"
	ws, err = websocket.Dial(url, "", origin)
	if err != nil {
		return
	}

	input = make(chan Message)
	go func() {
		for {
			var msg Message
			if err := websocket.JSON.Receive(ws, &msg); err != nil {
				close(input)
				return
			}
			if msg != nil {
				input <- msg
			}
		}
	}()

	output = make(chan Message)
	go func() {
		id := 0
		for {
			msg := <-output
			msg["id"] = id
			if err := websocket.JSON.Send(ws, msg); err != nil {
				close(output)
				return
			}
			id++
		}
	}()

	return
}

func main() {
	ws, input, output, err := connectToRtm()
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	messages := make(chan Message)
	go func() {
		for {
			select {
			case msg, ok := <-input:
				// Handle disconects.
				if !ok {
					log.Println("disconnected from rtm")
					ws, input, output, err = connectToRtm()
					if err != nil {
						log.Fatal(err)
					}
					defer ws.Close()
					continue
				}
				if msg.DirectMessage() {
					messages <- msg
				}
			case <-time.After(5 * time.Second):
				output <- Message{"type": "ping"}
			}
		}
	}()

	for m := range messages {
		log.Println(m)
		output <- Message{
			"type":    m.Type(),
			"channel": m.Channel(),
			"text":    m.Text(),
		}
	}
}
