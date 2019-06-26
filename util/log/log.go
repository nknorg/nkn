package log

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nknorg/nkn/util/config"
)

const (
	namePrefix = "LEVEL"
	callDepth  = 2
	mb         = 1024 * 1024
)

const (
	Red    = "0;31"
	Green  = "0;32"
	Yellow = "0;33"
	Pink   = "1;35"
)

const (
	debugLog = iota
	infoLog
	warnLog
	errorLog
	maxLevelLog
)

var (
	levels = map[int]string{
		debugLog: Color(Pink, "[DEBUG]"),
		infoLog:  Color(Green, "[INFO ]"),
		warnLog:  Color(Yellow, "[WARN ]"),
		errorLog: Color(Red, "[ERROR]"),
	}
	Stdout = os.Stdout
)

func Color(code, msg string) string {
	return fmt.Sprintf("\033[%sm%s\033[m", code, msg)
}

func GetGID() uint64 {
	var buf [64]byte
	b := buf[:runtime.Stack(buf[:], false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

var Log *Logger
var WebLog *Logger
var initOnce sync.Once

func LevelName(level int) string {
	if name, ok := levels[level]; ok {
		return name
	}
	return namePrefix + strconv.Itoa(level)
}

func NameLevel(name string) int {
	for k, v := range levels {
		if v == name {
			return k
		}
	}
	var level int
	if strings.HasPrefix(name, namePrefix) {
		level, _ = strconv.Atoi(name[len(namePrefix):])
	}
	return level
}

type Logger struct {
	sync.RWMutex
	level   int
	logger  *log.Logger
	logFile *os.File
}

func newLogger(out io.Writer, prefix string, flag, level int, file *os.File) *Logger {
	return &Logger{
		level:   level,
		logger:  log.New(out, prefix, flag),
		logFile: file,
	}
}

func (l *Logger) reset(out io.Writer, prefix string, flag, level int, file *os.File) {
	l.Lock()
	defer l.Unlock()
	l.closeLogFile()
	l.level = level
	l.logger = log.New(out, prefix, flag)
	l.logFile = file
}

func (l *Logger) SetDebugLevel(level int) error {
	if level >= maxLevelLog || level < 0 {
		return errors.New("Invalid Debug Level")
	}

	l.Lock()
	defer l.Unlock()

	l.level = level
	return nil
}

func (l *Logger) Output(level int, a ...interface{}) error {
	if l == nil {
		l = newLogger(Stdout, "", log.Ldate|log.Lmicroseconds, 0, nil)
	}

	l.RLock()
	defer l.RUnlock()

	if level >= l.level {
		gid := GetGID()
		gidStr := strconv.FormatUint(gid, 10)

		a = append([]interface{}{LevelName(level), "GID",
			gidStr + ","}, a...)

		return l.logger.Output(callDepth, fmt.Sprintln(a...))
	}
	return nil
}

func (l *Logger) Outputf(level int, format string, v ...interface{}) error {
	if l == nil {
		l = newLogger(Stdout, "", log.Ldate|log.Lmicroseconds, 0, nil)
	}

	l.RLock()
	defer l.RUnlock()

	if level >= l.level {
		gid := GetGID()
		v = append([]interface{}{LevelName(level), "GID",
			gid}, v...)

		return l.logger.Output(callDepth, fmt.Sprintf("%s %s %d, "+format+"\n", v...))
	}
	return nil
}

func (l *Logger) Debug(a ...interface{}) {
	l.Output(debugLog, a...)
}

func (l *Logger) Debugf(format string, a ...interface{}) {
	l.Outputf(debugLog, format, a...)
}

func (l *Logger) Info(a ...interface{}) {
	l.Output(infoLog, a...)
}

func (l *Logger) Infof(format string, a ...interface{}) {
	l.Outputf(infoLog, format, a...)
}

func (l *Logger) Warning(a ...interface{}) {
	l.Output(warnLog, a...)
}

func (l *Logger) Warningf(format string, a ...interface{}) {
	l.Outputf(warnLog, format, a...)
}

func (l *Logger) Error(a ...interface{}) {
	l.Output(errorLog, a...)
}

func (l *Logger) Errorf(format string, a ...interface{}) {
	l.Outputf(errorLog, format, a...)
}

func Debug(a ...interface{}) {
	Log.RLock()
	defer Log.RUnlock()

	if debugLog < Log.level {
		return
	}

	pc := make([]uintptr, 10)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	file, line := f.FileLine(pc[0])
	fileName := filepath.Base(file)

	nameFull := f.Name()
	nameEnd := filepath.Ext(nameFull)
	funcName := strings.TrimPrefix(nameEnd, ".")

	a = append([]interface{}{funcName + "()", fileName + ":" + strconv.Itoa(line)}, a...)

	Log.Debug(a...)
}

func Debugf(format string, a ...interface{}) {
	Log.RLock()
	defer Log.RUnlock()

	if debugLog < Log.level {
		return
	}

	pc := make([]uintptr, 10)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	file, line := f.FileLine(pc[0])
	fileName := filepath.Base(file)

	nameFull := f.Name()
	nameEnd := filepath.Ext(nameFull)
	funcName := strings.TrimPrefix(nameEnd, ".")

	a = append([]interface{}{funcName + "()", fileName, line}, a...)

	Log.Debugf("%s %s:%d "+format, a...)
}

func Info(a ...interface{}) {
	Log.Info(a...)
}

func Warning(a ...interface{}) {
	Log.Warning(a...)
}

func Error(a ...interface{}) {
	Log.Error(a...)
}

func Infof(format string, a ...interface{}) {
	Log.Infof(format, a...)
}

func Warningf(format string, a ...interface{}) {
	Log.Warningf(format, a...)
}

func Errorf(format string, a ...interface{}) {
	Log.Errorf(format, a...)
}

func FileOpen(path string, name string) (*os.File, error) {
	if fi, err := os.Stat(path); err == nil {
		if !fi.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", path)
		}
	} else {
		if err := os.MkdirAll(path, 0766); err != nil {
			return nil, err
		}
	}

	var currenttime string = time.Now().Format("2006-01-02_15.04.05")

	logfile, err := os.OpenFile(filepath.Join(path, currenttime+"_"+name+".log"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	return logfile, nil
}

func getWritterAndFile(name string, outputs ...interface{}) (io.Writer, *os.File, error) {
	writers := []io.Writer{}
	var logFile *os.File
	var err error
	if len(outputs) == 0 {
		writers = append(writers, ioutil.Discard)
	} else {
		for _, o := range outputs {
			switch o.(type) {
			case string:
				logFile, err = FileOpen(o.(string), name)
				if err != nil {
					return nil, nil, fmt.Errorf("open log file %v failed: %v", o, err)
				}
				writers = append(writers, logFile)
			case *os.File:
				writers = append(writers, o.(*os.File))
			default:
				return nil, nil, fmt.Errorf("invalid log location %v", o)
			}
		}
	}
	fileAndStdoutWrite := io.MultiWriter(writers...)
	return fileAndStdoutWrite, logFile, nil
}

func Init() error {
	var err error
	initOnce.Do(func() {
		var writter, webWritter io.Writer
		var file, webFile *os.File
		writter, file, err = getWritterAndFile("LOG", config.Parameters.LogPath, Stdout)
		if err != nil {
			return
		}
		webWritter, webFile, err = getWritterAndFile("WEBLOG", config.Parameters.LogPath)
		if err != nil {
			return
		}

		Log = newLogger(writter, "", log.Ldate|log.Lmicroseconds, config.Parameters.LogLevel, file)
		WebLog = newLogger(webWritter, "", log.Ldate|log.Lmicroseconds, config.Parameters.LogLevel, webFile)

		go func() {
			for {
				time.Sleep(config.ConsensusDuration)
				if Log.needNewLogFile() {
					writter, file, err = getWritterAndFile("LOG", config.Parameters.LogPath, Stdout)
					if err != nil {
						panic(err)
					}
					Log.reset(writter, "", log.Ldate|log.Lmicroseconds, config.Parameters.LogLevel, file)
				}
				if WebLog.needNewLogFile() {
					writter, file, err = getWritterAndFile("WEBLOG", config.Parameters.LogPath)
					if err != nil {
						panic(err)
					}
					WebLog.reset(writter, "", log.Ldate|log.Lmicroseconds, config.Parameters.LogLevel, file)
				}
			}
		}()
	})
	if err != nil {
		return err
	}
	return nil
}

func (l *Logger) GetLogFileSize() (int64, error) {
	l.RLock()
	defer l.RUnlock()

	f, e := l.logFile.Stat()
	if e != nil {
		return 0, e
	}
	return f.Size(), nil
}

func (l *Logger) needNewLogFile() bool {
	logFileSize, err := l.GetLogFileSize()
	maxLogFileSize := int64(config.Parameters.MaxLogFileSize) * mb
	if err != nil {
		return false
	}
	if logFileSize > maxLogFileSize {
		return true
	}
	return false
}

func (l *Logger) closeLogFile() error {
	var err error
	if l.logFile != nil {
		err = l.logFile.Close()
	}
	return err
}
