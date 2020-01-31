package actions

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Burmudar/gomodoro/middleware"
	"github.com/Burmudar/gomodoro/models"
	"github.com/Burmudar/gomodoro/pomodoro"
	"github.com/gobuffalo/buffalo"
	"github.com/gofrs/uuid"
	"github.com/gorilla/websocket"
)

var ErrNotFound error = fmt.Errorf("Item was not found")
var ErrInvalidContext error = fmt.Errorf("Context is not a WebsocketContext")

type WebSocketClientHandler struct {
	Key          string
	Ctx          *middleware.WebSocketContext
	Conn         *websocket.Conn
	TimerManager *pomodoro.TimerManager
	shutdownCh   chan bool
	logger       buffalo.Logger
}

type WebSocketHandlerStore struct {
	handlers map[string]*WebSocketClientHandler
	lock     sync.Mutex
}

func NewWebSocketClientHandler(ctx *middleware.WebSocketContext) *WebSocketClientHandler {
	var handler = WebSocketClientHandler{
		Key:          "",
		Ctx:          ctx,
		Conn:         ctx.Ws,
		TimerManager: pomodoro.NewTimerManager(),
		shutdownCh:   make(chan bool),
		logger:       ctx.Logger(),
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

var IdentifyType MsgType = "identify"
var IdentifiedType MsgType = "identified"
var RegisterType MsgType = "register"
var RegistrationIdType MsgType = "registration_id"
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

type Identify struct {
	Msg
	Key string
}

type IdentifiedReply struct {
	Msg
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
		ctx.Logger().Fatalf("Context is not of type WebSocketContext")
		return ErrInvalidContext
	}

	h := NewWebSocketClientHandler(&ctx)

	store.Add(h)

	go h.handleClient()
	return nil
}

func (h *WebSocketClientHandler) cleanup() {
	h.logger.Printf("%v: Notifying go routines to shutdown", h.Key)
	h.shutdownCh <- true
	h.logger.Printf("%v: Stopping all timers", h.Key)
	h.TimerManager.StopAll()
	h.logger.Printf("%v: Removed client from store", h.Key)
}

func (h *WebSocketClientHandler) listenMessages() chan Msg {
	msgChan := make(chan Msg)

	go func() {
		defer h.logger.Printf("Socket listener shutdown")

		for {
			msgType, data, err := h.Conn.ReadMessage()

			if err != nil {
				h.logger.Printf("%v: Websocket Error '%v'. Shutting down socket listener", h.Key, err)
				h.cleanup()
				return
			}

			switch msgType {
			case websocket.BinaryMessage:
				{
					h.logger.Printf("Cannot decode Binary Msg!")
				}
				break
			case websocket.TextMessage:
				{
					if msg, err := decodeTxtMsg(h.Ctx, string(data)); err != nil {
						h.logger.Error("Failed to decode msg: %v", err)
					} else {
						h.logger.Printf("Sending decoded msg to channel: %v", msg)
						msgChan <- msg
					}
				}
				break
			case websocket.CloseMessage:
				{
					h.logger.Printf("%v: Close SocketControlMessage received. Shutting down socket listener", h.Key)
					h.cleanup()
				}
			case websocket.PingMessage, websocket.PongMessage:
				fallthrough
			default:
				{
					h.logger.Printf("Received Msg Type: %v", string(msgType))
					h.logger.Printf("Data: %v", string(data))
				}
			}
		}

	}()
	return msgChan
}

func (h *WebSocketClientHandler) handleClient() {

	h.logger.Printf("Handling messages for client [%s]", h.Key)

	messages := h.listenMessages()

	for {
		select {
		case msg := <-messages:
			h.processMsg(msg)
		case <-h.shutdownCh:
			close(messages)
			h.logger.Printf("%v: Client message handler shutdown", h.Key)
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
	h.logger.Printf("Processing msg: %v", msg)

	if h.Key == "" {
		h.handleUnknownClient(msg)
	}

	switch msg.Type {
	case NewTimerType:
		h.logger.Printf("Handle New Timer msg")
		h.handleNewTimer(msg)
	case StartTimerType:
		h.logger.Printf("Handle Start Timer msg")
		h.handleStartTimer(msg)
	}

}

func (h *WebSocketClientHandler) handleNewTimer(msg Msg) {
	newTimerMsg := NewTimer{
		Msg:      msg,
		Interval: int(msg.fields["interval"].(float64)),
		Focus:    int(msg.fields["focus"].(float64)),
	}

	c, err := models.NewTimerConfig(time.Duration(newTimerMsg.Focus)*time.Minute, 5*time.Minute, time.Duration(newTimerMsg.Interval)*time.Second)

	if err != nil {
		h.logger.Errorf("Failed to create timer config: %v", err)
	}

	if err := models.DB.Create(c); err != nil {
		h.logger.Errorf("Failed to save timer config in database: %v", err)
	}

	config := &pomodoro.Config{
		TimerConfig: c,
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

func (h *WebSocketClientHandler) handleUnknownClient(msg Msg) {
	switch msg.Type {
	case IdentifyType:
		{
			identityMsg := Identify{
				msg,
				msg.fields["clientId"].(string),
			}
			h.logger.Printf("Handling Identify msg")
			h.handleIdentifyClient(identityMsg)
		}
		break
	case RegisterType:
		h.logger.Printf("Handling register msg")
		h.handleClientRegister(&Register{msg})
	}

}

func (h *WebSocketClientHandler) handleIdentifyClient(i Identify) {
	timerClient := models.TimerClient{}
	err := models.DB.Find(&timerClient, uuid.FromStringOrNil(i.Key))

	if err != nil {
		h.logger.Errorf("Failed to retrieve client with id: %v", i.Key)
		errorReply(h.Ctx, fmt.Sprintf("Client[%v] not found"))
	}

	h.Key = timerClient.ID.String()
	h.logger.Printf("Client[%v] identified!")

	reply := IdentifiedReply{
		Msg: Msg{
			Type:      IdentifiedType,
			Timestamp: time.Now().Unix(),
		},
	}

	h.logger.Debugf("Sending client identified reply")
	if err = h.Conn.WriteJSON(&reply); err != nil {
		h.logger.Errorf("Failed to send identified reply: %v", reply)
	}

}

func (h *WebSocketClientHandler) handleClientRegister(r *Register) {
	tc, err := models.NewTimerClient()
	if err != nil {
		h.logger.Errorf("Failed to create new TimerClient")
	}

	err = models.DB.Create(tc)
	if err != nil {
		h.logger.Errorf("Failed to save TimerClient in database")
	}

	h.Key = tc.ID.String()

	reply := RegisterReply{
		Msg: Msg{Type: RegistrationIdType,
			Timestamp: time.Now().Unix(),
		},
		Key: tc.ID.String(),
	}

	h.logger.Printf("Sending registration reply [%v]", h.Key)

	if err := h.Conn.WriteJSON(reply); err != nil {
		h.logger.Errorf("Failed to write registration reply: %v clientKey: %v", err, h.Key)
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
		h.logger.Errorf("Failed to start timer: %v", err)
	}

}

func handleBinaryMsg(ws *websocket.Conn, binary []byte) ([]byte, error) {
	return nil, nil
}
