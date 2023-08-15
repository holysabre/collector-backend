package logger

import (
	"log"
)

func ExitIfErr(err error, msg string) {
	if err != nil {
		log.Fatalf("%s, err: %v \n", msg, err.Error())
	}
}

func LogIfErr(err error) {
	if err != nil {
		log.Printf("%s \n", err.Error())
	}
}

func Fatal(msg string) {
	log.Fatal(msg)
}

func LogIfErrWithMsg(err error, msg string) {
	if err != nil {
		log.Printf("%s, err: %v \n", msg, err.Error())
	}
}

func Printf(format string, v ...any) {
	log.Printf(format, v...)
}

func Println(msg string) {
	log.Println(msg)
}
