package actions

import (
	"log"
	"time"

	"github.com/Burmudar/gomodoro/middleware"
	"github.com/gobuffalo/buffalo"
	"github.com/gorilla/websocket"
)

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

func handleTextMsg(wctx middleware.WebSocketContext, data string) (string, error) {

	wctx.Logger().Printf("Received from client: %v", data)

	timeData, err := time.Now().MarshalText()
	if err != nil {
		wctx.Logger().Errorf("Failed to marshall time to text: %v", err)
	}

	wctx.Ws.WriteMessage(websocket.TextMessage, timeData)

	wctx.Logger().Printf("Sent: %v", string(timeData))
	return "", nil
}

func handleBinaryMsg(ws *websocket.Conn, binary []byte) ([]byte, error) {

	return nil, nil
}
