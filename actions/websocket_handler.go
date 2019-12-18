package actions

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Burmudar/gomodoro/middleware"
	"github.com/Burmudar/gomodoro/pomodoro"
	"github.com/gobuffalo/buffalo"
	"github.com/gorilla/websocket"
)

var timerManager *pomodoro.TimerManager = pomodoro.NewTimerManager()

var RegisterType MsgType = "register"
var RegistratiodIdType MsgType = "registration_id"
var ErrorType MsgType = "error"

type MsgType string

type Msg struct {
	Type      MsgType   `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func WebsocketHandler(c buffalo.Context) error {
	ctx, ok := c.(middleware.WebSocketContext)
	if !ok {
		log.Fatalln("Context is not of type WebSocketContext")
	}
	for {
		msgType, data, err := ctx.Ws.ReadMessage()
		logger := ctx.Logger()

		if err != nil {
			logger.Error(err)
			return nil
		}

		switch msgType {
		case websocket.BinaryMessage:
			{
				handleBinaryMsg(ctx.Ws, data)
			}
			break
		case websocket.TextMessage:
			{
				handleTextMsg(ctx, string(data))
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
	return nil
}

func errorReply(wctx *middleware.WebSocketContext, message string) error {
	msg := Msg{
		Type:      ErrorType,
		Message:   message,
		Timestamp: time.Now(),
	}

	reply, err := json.Marshal(msg)
	if err != nil {
		wctx.Logger().Errorf("Failed to marshall error reply: %v", msg)
	}
	wctx.Ws.WriteMessage(websocket.TextMessage, reply)

	return nil
}

func handleTextMsg(wctx middleware.WebSocketContext, data string) (string, error) {

	wctx.Logger().Printf("Received from client: %v", data)

	var msg Msg

	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		wctx.Logger().Errorf("Failed to unmarshall data received from client: %v", data)
		errorReply(&wctx, "Error unmarshalling client data")
		return "", err
	}

	reply, err := handleMsg(msg)
	if err != nil {
		wctx.Logger().Errorf("Error occured while handling msg: %v error: %v", msg, err)
		errorReply(&wctx, fmt.Sprintf("Error occured while handling message: %v", msg))
	}

	if err := wctx.Ws.WriteJSON(reply); err != nil {
		wctx.Logger().Errorf("Failed to write JSON reply: %v error: %v", reply, err)
	}

	return "", nil
}

func handleMsg(msg Msg) (Msg, error) {
	var reply Msg
	var err error = nil

	switch msg.Type {
	case RegisterType:
		reply, err = handleClientRegister(msg)
	}

	return reply, err
}

func handleClientRegister(msg Msg) (Msg, error) {
	key := timerManager.NewTimer(&pomodoro.Config{})

	return Msg{
		RegistratiodIdType,
		fmt.Sprintf("%x", key),
		time.Now(),
	}, nil

}

func handleBinaryMsg(ws *websocket.Conn, binary []byte) ([]byte, error) {

	return nil, nil
}
