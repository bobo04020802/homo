//
// Copyright (c) 2019-present Codist <countstarlight@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// Written by Codist <countstarlight@gmail.com>, April 2019
//

package view

//go:generate go-bindata -pkg $GOPACKAGE -o bindata.go -prefix assets/ assets/...

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/countstarlight/homo/cmd/webview/config"
	"github.com/countstarlight/homo/module/baidu"
	"github.com/countstarlight/homo/module/com"
	"github.com/countstarlight/homo/module/nlu"
	"github.com/sirupsen/logrus"
	"github.com/zserge/webview"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

var (
	w webview.WebView
)

func InitWebView(title string, debug bool) {
	w = webview.New(webview.Settings{
		Width:                  900,
		Height:                 700,
		Resizable:              true,
		Title:                  title,
		URL:                    startServer(),
		Debug:                  debug,
		ExternalInvokeCallback: handleRPC,
	})
}

// Run webview
func Run() {
	w.Run()
}

func startServer() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		defer com.IOClose("webview ln", ln)
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if len(path) > 0 && path[0] == '/' {
				path = path[1:]
			}
			if path == "" {
				path = "index.html"
			}
			//fmt.Printf("path %s\n", path)
			if bs, err := Asset(path); err != nil {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.Header().Add("Content-Type", mime.TypeByExtension(filepath.Ext(path)))
				_, err = io.Copy(w, bytes.NewBuffer(bs))
				if err != nil {
					panic(err)
				}
			}
		})
		log.Fatal(http.Serve(ln, nil))
	}()
	return "http://" + ln.Addr().String()
}

type Message struct {
	Says []string `json:"says"`
}
type HomoReply struct {
	Msg Message `json:"message"`
}

func TypingAnimate() {
	w.Dispatch(func() {
		err := w.Eval("chatWindow.think()")
		if err != nil {
			logrus.Warning("botTypingAnimate w.Eval failed: %s", err.Error())
		}
	})
}

func TypingAnimateStop() {
	w.Dispatch(func() {
		err := w.Eval("chatWindow.thinkStop()")
		if err != nil {
			logrus.Warning("botTypingAnimateStop w.Eval failed: %s", err.Error())
		}
	})
}

func SendInputText(message string) {
	w.Dispatch(func() {
		err := w.Eval(fmt.Sprintf("chatWindow.InputText(\"%s\")", message))
		if err != nil {
			logrus.Warning("SendInputText: w.Eval failed: %s", err.Error())
		}
	})
}

// Only send text to input, not trigger thinking
func SendOnlyInputText(message string) {
	w.Dispatch(func() {
		err := w.Eval(fmt.Sprintf("chatWindow.OnlyInputText(\"%s\")", message))
		if err != nil {
			logrus.Warning("SendOnlyInputText: w.Eval failed: %s", err.Error())
		}
	})
}

func SendReply(message []string) {
	b, err := json.Marshal(HomoReply{
		Msg: Message{
			Says: message,
		},
	})
	if err != nil {
		logrus.Warning("SendReply: json.Marshal failed: %s", err.Error())
	}
	w.Dispatch(func() {
		err = w.Eval(fmt.Sprintf("chatWindow.talk(%s, \"message\")", string(b)))
		if err != nil {
			logrus.Warning("SendReply: w.Eval failed: %s", err.Error())
		}
	})
}

func SendReplyWithVoice(message []string) {
	SendReply(message)
	//Play voice
	time.Sleep(time.Second)
	config.IsPlayingVoice = true
	for _, sent := range message {
		config.VoicePlayMutex.Lock()
		err := baidu.TextToSpeech(sent)
		config.VoicePlayMutex.Unlock()
		if err != nil {
			SendReply([]string{"语音合成出错: " + err.Error()})
		}
	}
	config.IsPlayingVoice = false
}

func handleRPC(w webview.WebView, data string) {
	switch {
	case strings.HasPrefix(data, "message:"):
		msg := strings.TrimPrefix(data, "message:")
		//fmt.Printf("发送的消息: %s\n", msg)
		//go TypingAnimate()
		go func() {
			var reply []string
			replyMessage, err := nlu.ActionLocal(msg)
			if err != nil {
				reply = []string{"错误: " + err.Error()}
			} else {
				reply = replyMessage
			}
			if !config.OfflineMode {
				SendReplyWithVoice(reply)
			} else {
				SendReply(reply)
			}
		}()
	}
}
