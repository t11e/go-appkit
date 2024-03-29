package appkit

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jmcvetta/randutil"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
)

const contextLoggerKey = "reqLogger"

func WrapLoggingHandler(handler ContextHandlerFunc) ContextHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		loggingW := wrapLoggingResponseWriter(w)

		logger := newLoggerForId(makeId())
		ctx = context.WithValue(ctx, contextLoggerKey, logger)

		t := time.Now()
		writeStartLine(logger, req, t, params)

		handler(ctx, loggingW, req, params)

		t2 := time.Now()
		writeEndLine(logger, req, t2, loggingW.Status(), loggingW.Size(), t2.Sub(t))
	}
}

func GetLoggerFromContext(ctx context.Context) *log.Logger {
	if logger, ok := ctx.Value(contextLoggerKey).(*log.Logger); ok {
		return logger
	}
	return log.New(os.Stdout, "", 0)
}

func newLoggerForId(id string) *log.Logger {
	return log.New(os.Stdout, fmt.Sprintf("[%s] ", id), 0)
}

func makeId() string {
	if r, err := randutil.AlphaString(8); err == nil {
		return fmt.Sprintf("%s%x", r, time.Now().Unix())
	} else {
		panic(err)
	}
}

func writeStartLine(
	logger *log.Logger,
	req *http.Request,
	timestamp time.Time,
	params httprouter.Params) {
	buf := new(bytes.Buffer)
	buf.WriteString("Handling ")
	buf.WriteString(req.Method)
	buf.WriteString(" ")
	buf.WriteString(req.URL.String())

	if len(params) > 0 {
		buf.WriteString(" ")
		for i, param := range params {
			if i > 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(param.Key)
			buf.WriteString("=")
			buf.WriteString(fmt.Sprintf("%v", param.Value))
		}
	}

	logger.Print(buf.String())
}

func writeEndLine(
	logger *log.Logger,
	req *http.Request,
	timestamp time.Time,
	status int,
	size int,
	elapsedTime time.Duration) {
	logger.Printf("Completed %s %s (%d, %dms, %d bytes)", req.Method, req.URL.String(),
		status, int(elapsedTime/time.Millisecond), size)
}

// The following derived from https://github.com/gorilla/handlers/blob/master/handlers.go
// Copyright (c) 2013 The Gorilla Handlers Authors. All rights reserved.

type loggingResponseWriter interface {
	http.ResponseWriter
	http.Flusher
	Status() int
	Size() int
}

func wrapLoggingResponseWriter(w http.ResponseWriter) loggingResponseWriter {
	var logger loggingResponseWriter = &responseLogger{w: w}
	if _, ok := w.(http.Hijacker); ok {
		logger = &hijackLogger{responseLogger{w: w}}
	}
	h, ok1 := logger.(http.Hijacker)
	c, ok2 := w.(http.CloseNotifier)
	if ok1 && ok2 {
		return hijackCloseNotifier{logger, h, c}
	}
	if ok2 {
		return &closeNotifyWriter{logger, c}
	}
	return logger
}

type responseLogger struct {
	w      http.ResponseWriter
	status int
	size   int
}

func (l *responseLogger) Header() http.Header {
	return l.w.Header()
}

func (l *responseLogger) Write(b []byte) (int, error) {
	if l.status == 0 {
		// The status will be StatusOK if WriteHeader has not been called yet
		l.status = http.StatusOK
	}
	size, err := l.w.Write(b)
	l.size += size
	return size, err
}

func (l *responseLogger) WriteHeader(s int) {
	l.w.WriteHeader(s)
	l.status = s
}

func (l *responseLogger) Status() int {
	return l.status
}

func (l *responseLogger) Size() int {
	return l.size
}

func (l *responseLogger) Flush() {
	f, ok := l.w.(http.Flusher)
	if ok {
		f.Flush()
	}
}

type hijackLogger struct {
	responseLogger
}

func (l *hijackLogger) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h := l.responseLogger.w.(http.Hijacker)
	conn, rw, err := h.Hijack()
	if err == nil && l.responseLogger.status == 0 {
		// The status will be StatusSwitchingProtocols if there was no error and WriteHeader has not been called yet
		l.responseLogger.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

type closeNotifyWriter struct {
	loggingResponseWriter
	http.CloseNotifier
}

type hijackCloseNotifier struct {
	loggingResponseWriter
	http.Hijacker
	http.CloseNotifier
}
