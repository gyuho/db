package main

import (
	"log"

	"github.com/gyuho/db/xlog"
)

var logger = xlog.NewLogger("example", xlog.INFO)

func main() {
	logger.SetMaxLogLevel(xlog.DEBUG)

	log.Println("Hello World!")
	logger.Println("Hello World!")
	logger.Debugln("Hello World!")
}

/*
... I | Hello World!
... I | example: Hello World!
... D | example: Hello World!
*/
