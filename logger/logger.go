package logger

import (
	"log"
	"os"
)

type Logger struct {
	log   *log.Logger
	warn  *log.Logger
	err   *log.Logger
	trace *log.Logger
}

func New(prefix string) Logger {
	return Logger{
		log:   log.New(os.Stdout, "["+prefix+"] ", log.Ldate|log.Ltime),
		warn:  log.New(os.Stderr, "["+prefix+" WARN] ", log.Ldate|log.Ltime|log.Lshortfile),
		err:   log.New(os.Stderr, "["+prefix+" ERR] ", log.Ldate|log.Ltime|log.Llongfile),
		trace: log.New(os.Stderr, "["+prefix+" TRACE] ", log.Ldate|log.Ltime|log.Llongfile),
	}
}

func (l Logger) Log(format string, a ...interface{}) {
	l.log.Printf(format, a...)
}
func (l Logger) Warn(format string, a ...interface{}) {
	l.warn.Printf(format, a...)
}
func (l Logger) Err(err error, format string, a ...interface{}) {
	if err != nil {
		l.err.Printf(format+","+err.Error(), a...)
	} else {
		l.err.Printf(format, a...)
	}
}
func (l Logger) Trace(format string, a ...interface{}) {
	l.trace.Printf(format, a...)
}
