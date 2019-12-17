package middleware

import (
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/gorilla/websocket"
)

type WebSocketContext struct {
	buffalo.Context
	Ws *websocket.Conn
}

type ValidateOriginFn func(r *http.Request) bool

var DefaultValidateOrigin ValidateOriginFn = AllOriginValid

func AllOriginValid(r *http.Request) bool {
	return true
}

func Websockets(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     DefaultValidateOrigin,
		}

		c.Logger().Printf("Attempting to upgrade connection to websockets")
		conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)

		if err != nil {
			errMsg := fmt.Sprintf("Failed to upgrade connection to websockets: %v", err)
			c.Logger().Errorf(errMsg)

			resp := struct {
				Success bool
				Message string
			}{false, errMsg}

			c.Render(500, render.JSON(resp))
		}

		c.Logger().Printf("Connection upgraded! Welcome to Websocket land!")

		return next(WebSocketContext{c, conn})
	}
}
