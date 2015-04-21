package rtm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/net/websocket"
)

var (
	origin  = "http://localhost/"
	timeout = 5 * time.Second
)

type Message map[string]interface{}

func NewResponse(msg Message, text string) Message {
	return Message{
		"type":    "message",
		"channel": msg["channel"],
		"text":    text,
	}
}

func (m Message) String() string {
	buf, _ := json.Marshal(m)
	return string(buf)
}

type Stream struct {
	token  string
	ws     *websocket.Conn
	Input  chan Message
	Output chan Message
}

func LoopForever(token string, onMessage func(Message, chan Message)) error {
	s, err := Connect(token)
	if err != nil {
		return err
	}
	defer s.Close()

	for {
		select {
		case msg, ok := <-s.Input:
			// Handle disconects.
			if !ok {
				s, err = Connect(token)
				if err != nil {
					return errors.New(fmt.Sprint("unable to reconnect:", err))
				}
				defer s.Close()
				continue
			}
			go onMessage(msg, s.Output)
		case <-time.After(timeout):
			s.Output <- Message{"type": "ping"}
		}
	}
}

func Connect(token string) (s *Stream, err error) {
	s = new(Stream)
	s.token = token

	var url string
	url, err = s.webSocketUrl()
	if err != nil {
		return
	}

	s.ws, err = websocket.Dial(url, "", origin)
	if err != nil {
		return
	}

	s.Input = make(chan Message)
	go func() {
		for {
			var msg Message
			if err := websocket.JSON.Receive(s.ws, &msg); err != nil {
				close(s.Input)
				return
			}
			if msg != nil {
				s.Input <- msg
			}
		}
	}()

	s.Output = make(chan Message)
	go func() {
		id := 0
		for {
			msg := <-s.Output
			msg["id"] = id
			if err := websocket.JSON.Send(s.ws, msg); err != nil {
				close(s.Output)
				return
			}
			id++
		}
	}()

	return
}

func (s *Stream) webSocketUrl() (url string, err error) {
	resp, err := http.Get("https://slack.com/api/rtm.start?token=" + s.token)
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

func (s *Stream) Close() error {
	return s.ws.Close()
}
