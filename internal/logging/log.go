package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"
)

type LogType string

const (
	Access LogType = LogType("access")
	Error  LogType = LogType("error")
)

var (
	accessLog *Logger
	errorLog  *Logger
)

func GetLogger(logType LogType) *Logger {
	switch logType {
	case Access:
		if accessLog != nil {
			return accessLog
		}
		// Create access logger
		accessLog = createLogger(logType)
		return accessLog
	case Error:
		if errorLog != nil {
			return errorLog
		}
		// Create error logger
		errorLog = createLogger(logType)
		return errorLog
	default:
		panic("Logging failed")
	}
}

func createLogger(logType LogType) *Logger {
	var logFile *os.File
	switch runtime.GOOS {
	case "windows":
		logFile = createLocalLog(logType)
	case "darwin":
		logFile = createLocalLog(logType)
	default:
		// Check if /var/log/DMRHub exists
		// If not, create it. If we don't have permission
		// to create it, then create a local log file
		file := fmt.Sprintf("/var/log/DMRHub/DMRHub.%s.log", logType)
		if _, err := os.Stat("/var/log/DMRHub"); os.IsNotExist(err) {
			err := os.Mkdir("/var/log/DMRHub", 0755)
			if err != nil {
				logFile = createLocalLog(logType)
				break
			}
			err = os.Chown("/var/log/DMRHub", os.Getuid(), os.Getgid())
			if err != nil {
				logFile = createLocalLog(logType)
				break
			}

			logFile, err = os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0665)
			if err != nil {
				logFile = createLocalLog(logType)
				break
			}
		} else {
			logFile, err = os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0665)
			if err != nil {
				logFile = createLocalLog(logType)
				break
			}
		}
	}

	var sysLogger *log.Logger
	switch logType {
	case Access:
		sysLogger = log.New(logFile, "", log.LstdFlags)
	case Error:
		sysLogger = log.New(io.MultiWriter(os.Stderr, logFile), "", log.LstdFlags)
	}

	logger := &Logger{
		logger:  sysLogger,
		file:    logFile,
		Writer:  sysLogger.Writer(),
		channel: make(chan string, 200),
	}

	go logger.Relay()

	return logger
}

func (l *Logger) Relay() {
	for {
		select {
		case msg, ok := <-l.channel:
			if !ok {
				return
			}
			l.logger.Print(msg)
		}
	}
}

func createLocalLog(logType LogType) *os.File {
	file := fmt.Sprintf("DMRHub.%s.log", logType)
	logFile, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0665)
	if err != nil {
		log.Fatalf("Failed to create log file: %s:\n%v", file, err)
	}
	return logFile
}

type Logger struct {
	logger  *log.Logger
	file    *os.File
	Writer  io.Writer
	channel chan string
}

// Pass the function itself to the logger
func (l *Logger) Log(function interface{}, format string) {
	l.channel <- fmt.Sprintf("%s: %s", getFunctionName(function), format)
}

func (l *Logger) Logf(function interface{}, format string, args ...interface{}) {
	l.channel <- fmt.Sprintf("%s: %s", getFunctionName(function), fmt.Sprintf(format, args...))
}

// Use a tiny bit of reflection to get the name of the function
func getFunctionName(i interface{}) string {
	name := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	return strings.TrimPrefix(name, "github.com/USA-RedDragon/DMRHub/")
}

func Close() {
	close(accessLog.channel)
	close(errorLog.channel)
	_ = accessLog.file.Close()
	_ = errorLog.file.Close()
}