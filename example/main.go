package main

import (
	"github.com/evangwt/go-vncproxy"
	"github.com/gin-gonic/gin"
	"net/http"
)

func main() {
	r := gin.Default()

	vncProxy := NewVNCProxy()
	r.GET("/ws", func(ctx *gin.Context) {
		vncProxy.ServeWS(ctx.Writer, ctx.Request)
	})

	if err := r.Run(); err != nil {
		panic(err)
	}
}

func NewVNCProxy() *vncproxy.Proxy {
	return vncproxy.New(&vncproxy.Config{
		LogLevel: vncproxy.DebugLevel,
		// Logger: customerLogger,    // inject a custom logger
		// DialTimeout: 10 * time.Second, // customer DialTimeout
		TokenHandler: func(r *http.Request) (addr string, err error) {
			return ":5901", nil
		},
	})
}
