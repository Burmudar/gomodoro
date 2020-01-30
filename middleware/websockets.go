package middleware

import (
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo/render"
	"github.com/Burmudar/gomodoro/models"
	"github.com/gorilla/websocket"
	"github.com/gofrs/uuid"

)

type WebSocketContext struct {
	buffalo.Context
	Ws *websocket.Conn
	UUID uuid.UUID
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

		tc, err := models.NewTimerClient()
		if err != nil {
			c.Logger().Errorf("Failed to create new TimerClient")
		}

		err = models.DB.Create(tc)
		if err != nil {
			c.Logger().Errorf("Failed to save TimerClient in database")
		}

		return next(WebSocketContext{c, conn, tc.ID})
	}
}
