package grifts

import (
	"github.com/Burmudar/gomodoro/actions"
	"github.com/gobuffalo/buffalo"
)

func init() {
	buffalo.Grifts(actions.App())
}
