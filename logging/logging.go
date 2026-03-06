package logging

import (
	"log"
	"os"
	"sync"
)

var (
	Info  *log.Logger
	Error *log.Logger
	once  sync.Once
)

func Init() {
	once.Do(func() {
		Info = log.New(os.Stdout, "[INFO] ", log.LstdFlags)
		Error = log.New(os.Stderr, "[ERROR] ", log.LstdFlags)
	})
}

func InfoMsg(format string, v ...interface{}) {
	if Info == nil {
		Init()
	}
	Info.Printf(format, v...)
}

func ErrorMsg(format string, v ...interface{}) {
	if Error == nil {
		Init()
	}
	Error.Printf(format, v...)
}
