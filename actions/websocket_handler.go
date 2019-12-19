package actions

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/Burmudar/gomodoro/middleware"
	"github.com/Burmudar/gomodoro/pomodoro"
	"github.com/gobuffalo/buffalo"
	"github.com/gorilla/websocket"
)

var ErrNotFound error = fmt.Errorf("Item was not found")

type WebSocketClientHandler struct {
	Key          string
	Ctx          *middleware.WebSocketContext
	Conn         *websocket.Conn
	TimerManager *pomodoro.TimerManager
	shutdownCh   chan bool
}

type WebSocketHandlerStore struct {
	handlers map[string]*WebSocketClientHandler
	lock     sync.Mutex
}

func NewWebSocketClientHandler(ctx *middleware.WebSocketContext) *WebSocketClientHandler {
	var handler = WebSocketClientHandler{
		Key:        fmt.Sprintf("%x", rand.Int31()),
		Ctx:        ctx,
		Conn:       ctx.Ws,
		shutdownCh: make(chan bool),
	}

	return &handler
}

func NewWebSocketHandlerStore() *WebSocketHandlerStore {
	var store = WebSocketHandlerStore{
		handlers: make(map[string]*WebSocketClientHandler),
	}

	return &store
}

func (s *WebSocketHandlerStore) Add(h *WebSocketClientHandler) {
	s.lock.Lock()
	s.handlers[h.Key] = h
	defer s.lock.Unlock()
}

func (s *WebSocketHandlerStore) Del(key string) {
	s.lock.Lock()
	delete(s.handlers, key)
	s.lock.Unlock()
}

func (s *WebSocketHandlerStore) Get(key string) (*WebSocketClientHandler, error) {
	handler, ok := s.handlers[key]

	if !ok {
		return nil, ErrNotFound
	}

	return handler, nil
}

var store = NewWebSocketHandlerStore()

var RegisterType MsgType = "register"
var RegistratiodIdType MsgType = "registration_id"
var NewTimerType MsgType = "new_timer"
var TimerCreatedType MsgType = "timer_created"
var StartTimerType MsgType = "start_timer"
var TimerIntervalEventType MsgType = "timer_interval_event"
var TimerCompleteEventType MsgType = "timer_complete_event"
var ErrorType MsgType = "error"

type MsgType string

type Msg struct {
	Type      MsgType                `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	fields    map[string]interface{} `json:"omitempty"`
}

type Register struct {
	Msg
}

type RegisterReply struct {
	Msg
	Key string
}

type NewTimer struct {
	Msg
	Interval int `json:"interval"`
	Focus    int `json:"focus"`
}

type TimerCreatedReply struct {
	Msg
	TimerId string `json:"timerId"`
}

type StartTimer struct {
	Msg
	TimerId string `json:"timerId"`
}

type TimerEvent struct {
	Msg
	TimerId string `json:"timerId"`
	Elapsed int    `json:"elapsed"`
}

func WebsocketHandler(c buffalo.Context) error {
	ctx, ok := c.(middleware.WebSocketContext)
	if !ok {
		log.Fatalln("Context is not of type WebSocketContext")
	}

	h := NewWebSocketClientHandler(&ctx)

	store.Add(h)

	go h.handleClient()
	return nil
}

func (h *WebSocketClientHandler) listenMessages() chan Msg {
	msgChan := make(chan Msg)

	go func() {

		for {
			msgType, data, err := h.Conn.ReadMessage()
			logger := h.Ctx.Logger()

			if err != nil {
				logger.Error(err)
				defer close(msgChan)
				return
			}

			switch msgType {
			case websocket.BinaryMessage:
				{
					h.Ctx.Logger().Printf("Cannot decode Binary Msg!")
				}
				break
			case websocket.TextMessage:
				{
					if msg, err := decodeTxtMsg(h.Ctx, string(data)); err != nil {

					} else {
						msgChan <- msg
					}
				}
				break
			case websocket.CloseMessage:
				{
					//handle close message
				}
				break
			case websocket.PingMessage:
				fallthrough
			case websocket.PongMessage:
				fallthrough
			default:
				{
					logger.Printf("Received Msg Type: %v", string(msgType))
					logger.Printf("Data: %v", string(data))
				}
			}
		}

	}()
	return msgChan
}

func (h *WebSocketClientHandler) handleClient() {

	h.Ctx.Logger().Printf("Handling messages for client [%s]", h.Key)

	messages := h.listenMessages()

	select {
	case msg := <-messages:
		h.processMsg(msg)
	case <-h.shutdownCh:
		close(messages)
		break
	}
}

func errorReply(wctx *middleware.WebSocketContext, message string) error {
	msg := Msg{
		Type:      ErrorType,
		Timestamp: time.Now(),
	}

	reply, err := json.Marshal(msg)
	if err != nil {
		wctx.Logger().Errorf("Failed to marshall error reply: %v", msg)
	}
	wctx.Ws.WriteMessage(websocket.TextMessage, reply)

	return nil
}

func decodeTxtMsg(wctx *middleware.WebSocketContext, data string) (Msg, error) {
	var msg Msg

	wctx.Logger().Printf("decoding msg: %v", data)
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		wctx.Logger().Errorf("Failed to unmarshall data received from client: %v", data)
		errorReply(wctx, "Error unmarshalling client data")
		return msg, err
	}

	return msg, nil
}

func (h *WebSocketClientHandler) processMsg(msg Msg) {

	h.Ctx.Logger().Printf("Processing msg: %v", msg)

	switch msg.Type {
	case RegisterType:
		h.Ctx.Logger().Printf("Handling register msg")
		h.handleClientRegister(&Register{msg})
	case NewTimerType:
		h.Ctx.Logger().Printf("Handle New Timer msg")
		h.handleNewTimer(msg)
	}

}

func (h *WebSocketClientHandler) handleNewTimer(msg Msg) {
	newTimerMsg := NewTimer{
		Msg:      msg,
		Interval: msg.fields["interval"].(int),
		Focus:    msg.fields["focus"].(int),
	}

	config := &pomodoro.Config{
		FocusTime: time.Duration(newTimerMsg.Interval) * time.Minute,
		BreakTime: 5 * time.Minute,
		Interval:  time.Duration(newTimerMsg.Interval) * time.Second,
		IntervalCB: func() {
			h.Conn.WriteJSON(TimerEvent{
				Msg: Msg{
					TimerIntervalEventType,
					time.Now(),
					nil,
				},
				TimerId: fmt.Sprintf("%x", 1234),
			})
		},
		CompleteCB: func() {
			h.Conn.WriteJSON(TimerEvent{
				Msg: Msg{
					TimerCompleteEventType,
					time.Now(),
					nil,
				},
				TimerId: fmt.Sprintf("%x", 1234),
			})
		},
	}

	key := h.TimerManager.NewTimer(config)

	h.Conn.WriteJSON(TimerCreatedReply{
		Msg: Msg{
			TimerCreatedType,
			time.Now(),
			nil,
		},
		TimerId: fmt.Sprintf("%v", key),
	})
}

func (h *WebSocketClientHandler) handleClientRegister(r *Register) {
	reply := RegisterReply{
		Msg: Msg{Type: RegistratiodIdType,
			Timestamp: time.Now(),
		},
		Key: h.Key,
	}

	h.Ctx.Logger().Printf("Sending registration reply [%v]", h.Key)

	if err := h.Conn.WriteJSON(reply); err != nil {
		h.Ctx.Logger().Errorf("Failed to write registration reply: %v clientKey: %v", err, h.Key)
	}
	return
}

func handleBinaryMsg(ws *websocket.Conn, binary []byte) ([]byte, error) {
	return nil, nil
}
