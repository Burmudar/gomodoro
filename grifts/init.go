package grifts

import (
	"github.com/burmudar/gomodoro/actions"
	"github.com/gobuffalo/buffalo"
)

func init() {
	buffalo.Grifts(actions.App())
}
