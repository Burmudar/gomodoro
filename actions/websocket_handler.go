package actions

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"
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
		Key:          fmt.Sprintf("%x", rand.Int31()),
		Ctx:          ctx,
		Conn:         ctx.Ws,
		TimerManager: pomodoro.NewTimerManager(),
		shutdownCh:   make(chan bool),
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
	Timestamp int64                  `json:"timestamp"`
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
	TimerId int
}

type TimerEvent struct {
	Msg
	TimerId   string    `json:"timerId"`
	StartedAt time.Time `json:"startedAt"`
	EndsAt    time.Time `json:"endsAt"`
	Elapsed   float64   `json:"elapsed"`
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

			h.Ctx.Logger().Printf("Raw: %v %v %v", msgType, data, err)

			if err != nil {
				logger.Error(err)
				h.Ctx.Logger().Printf("CLOSING!")
				h.shutdownCh <- true
				close(h.shutdownCh)
				h.TimerManager.StopAll()
				store.Del(h.Key)
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
						h.Ctx.Logger().Error("Failed to decode msg: %v", err)
					} else {
						h.Ctx.Logger().Printf("Sending decoded msg to channel: %v", msg)
						msgChan <- msg
						h.Ctx.Logger().Printf("sent to channel!")
					}
				}
				break
			case websocket.CloseMessage:
				{
					h.Ctx.Logger().Printf("CLOSING!")
					h.shutdownCh <- true
					close(h.shutdownCh)
					h.TimerManager.StopAll()
					store.Del(h.Key)
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

	for {
		select {
		case msg := <-messages:
			h.processMsg(msg)
		case <-h.shutdownCh:
			close(messages)
			return
		}
	}
}

func errorReply(wctx *middleware.WebSocketContext, message string) error {
	msg := Msg{
		Type:      ErrorType,
		Timestamp: time.Now().Unix(),
	}

	wctx.Logger().Errorf("Error: %v", message)

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

	result := make(map[string]interface{})
	var err error

	if err = json.Unmarshal([]byte(data), &result); err != nil {
		wctx.Logger().Errorf("Failed to unmarshall data received from client: %v", data)
		errorReply(wctx, "Error unmarshalling client data")
	}

	if v, ok := result["type"]; ok {
		msg.Type = MsgType(v.(string))
	}
	if v, ok := result["timestamp]"]; ok {
		msg.Timestamp = v.(int64)
	}
	msg.fields = result

	return msg, nil
}

func (h *WebSocketClientHandler) processMsg(msg Msg) {

	if len(msg.fields) == 0 {
		return
	}
	h.Ctx.Logger().Printf("Processing msg: %v", msg)

	switch msg.Type {
	case RegisterType:
		h.Ctx.Logger().Printf("Handling register msg")
		h.handleClientRegister(&Register{msg})
	case NewTimerType:
		h.Ctx.Logger().Printf("Handle New Timer msg")
		h.handleNewTimer(msg)
	case StartTimerType:
		h.Ctx.Logger().Printf("Handle Start Timer msg")
		h.handleStartTimer(msg)
	}

}

func (h *WebSocketClientHandler) handleNewTimer(msg Msg) {
	newTimerMsg := NewTimer{
		Msg:      msg,
		Interval: int(msg.fields["interval"].(float64)),
		Focus:    int(msg.fields["focus"].(float64)),
	}

	config := &pomodoro.Config{
		FocusTime: time.Duration(newTimerMsg.Focus) * time.Minute,
		BreakTime: 5 * time.Minute,
		Interval:  time.Duration(newTimerMsg.Interval) * time.Second,
		IntervalCB: func(ts *pomodoro.TimerState) {
			h.Conn.WriteJSON(TimerEvent{
				Msg: Msg{
					TimerIntervalEventType,
					time.Now().Unix(),
					nil,
				},
				TimerId:   fmt.Sprintf("%x", 1234),
				StartedAt: ts.StartAt,
				EndsAt:    ts.EndsAt,
				Elapsed:   ts.Elapsed.Seconds(),
			})
		},
		CompleteCB: func(ts *pomodoro.TimerState) {
			h.Conn.WriteJSON(TimerEvent{
				Msg: Msg{
					TimerCompleteEventType,
					time.Now().Unix(),
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
			time.Now().Unix(),
			nil,
		},
		TimerId: fmt.Sprintf("%v", key),
	})
}

func (h *WebSocketClientHandler) handleClientRegister(r *Register) {
	reply := RegisterReply{
		Msg: Msg{Type: RegistratiodIdType,
			Timestamp: time.Now().Unix(),
		},
		Key: h.Key,
	}

	h.Ctx.Logger().Printf("Sending registration reply [%v]", h.Key)

	if err := h.Conn.WriteJSON(reply); err != nil {
		h.Ctx.Logger().Errorf("Failed to write registration reply: %v clientKey: %v", err, h.Key)
	}
	return
}

func (h *WebSocketClientHandler) handleStartTimer(msg Msg) {
	hexId := msg.fields["timerId"].(string)

	key, err := strconv.ParseInt(hexId, 10, 0)
	if err != nil {
		errorReply(h.Ctx, fmt.Sprintf("Failed to parse given timerId: %s", hexId))
		return
	}

	if err = h.TimerManager.StartTimer(int32(key)); err != nil {
		h.Ctx.Logger().Errorf("Failed to start timer: %v", err)
	}

}

func handleBinaryMsg(ws *websocket.Conn, binary []byte) ([]byte, error) {
	return nil, nil
}
