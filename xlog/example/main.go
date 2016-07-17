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

	lg, ok := xlog.GetLogger("example")
	if !ok {
		log.Fatal("'example' logger must exist")
	}
	lg.SetMaxLogLevel(xlog.INFO)
	logger.Debugln("DO NOT PRINT THIS")
	logger.Infoln("Done!")
}

/*
2016-07-17 15:37:36.741912 I | Hello World!
2016-07-17 15:37:36.741992 I | example: Hello World!
2016-07-17 15:37:36.742014 D | example: Hello World!
2016-07-17 15:37:36.742021 I | example: Done!
*/
