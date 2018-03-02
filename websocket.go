// websocket.go
package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type connection struct {
	// websocket 连接器
	ws *websocket.Conn
	// 发送信息的缓冲 channel
	send chan []byte
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func checkSameOrigin(r *http.Request) bool {
	return true
}

func websocketListen(done chan struct{}) {

	//////这种方法不能关闭//////
	//  http.HandleFunc("/ws", handelWebsocket)
	//	log.Printf("ws started")
	//	log.Fatal(http.ListenAndServe(*(gAppConfig.PushAddress), nil)) //阻塞

	///这种方法能关闭httpserver///
	http.HandleFunc("/ws", handelWebsocket)
	srvCLoser, err := ListenAndServeWithClose(gAppConfig.PushAddress, nil)
	if err != nil {
		log.Fatalln("ListenAndServeWithClose Error - ", err)
	} else {
		log.Printf("ws started")
	}
	done <- struct{}{}
	err = srvCLoser.Close()
	if err != nil {
		log.Fatalln("Server Close Error - ", err)
	}
	log.Println("Server Closed")
}

func handelWebsocket(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = checkSameOrigin
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	c := &connection{send: make(chan []byte, 1024), ws: ws}

	h.register <- c //此处先读取再确认身份，后期可以交互
	log.Printf("connect %s", c.ws.RemoteAddr())
	defer func() {
		h.unregister <- c
		log.Printf("disconnect %s", c.ws.RemoteAddr())
	}()
	go c.writer()
	c.reader()
}

func ListenAndServeWithClose(addr string, handler http.Handler) (sc io.Closer, err error) {

	var listener net.Listener

	srv := &http.Server{Addr: addr, Handler: handler}

	if addr == "" {
		addr = ":http"
	}

	listener, err = net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	go func() {
		err := srv.Serve(tcpKeepAliveListener{listener.(*net.TCPListener)})
		if err != nil {
			log.Println("HTTP Server Error - ", err)
		}
	}()

	return listener, nil
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func (c *connection) reader() {
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		log.Printf(string(message)) //以后可在这里做交互。 如果身份不正确，可以kick掉。
		//例如：登录协议，其他交互协议
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for message := range c.send {
		err := c.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
	c.ws.Close()
}
