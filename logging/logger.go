package logging

import (
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/howeyc/fsnotify"
)

var std = NewFileLogger()

func init() {
	logrus.SetFormatter(&TextFormatter{})

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP)

	go func() {
		for {
			select {
			case signal := <-signalChan:
				if signal == syscall.SIGHUP {
					err := std.Reopen()
					logrus.Infof("HUP received, reopen log %#v", std.Filename())
					if err != nil {
						logrus.Errorf("Reopen log %#v failed: %#s", std.Filename(), err.Error())
					}
				}

			case <-std.Watcher.Event:
				err := std.Reopen()
				logrus.Infof("Reopen log %#v by fsnotify event", std.Filename())
				if err != nil {
					logrus.Errorf("Reopen log %#v failed: %#s", std.Filename(), err.Error())
				}
			}
		}
	}()
}

// FileLogger wrapper
type FileLogger struct {
	sync.RWMutex
	filename string
	fd       *os.File
	Watcher  *fsnotify.Watcher
}

// NewFileLogger create instance FileLogger
func NewFileLogger() *FileLogger {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Warningf("fsnotify.NewWatcher(): %s", err)
		watcher = nil
	}
	return &FileLogger{
		filename: "",
		fd:       nil,
		Watcher:  watcher,
	}
}

// Open file for logging
func (l *FileLogger) Open(filename string) error {
	l.Lock()
	l.filename = filename
	l.Unlock()

	reopenErr := l.Reopen()

	if l.Watcher != nil {
		if err := l.Watcher.WatchFlags(filename, fsnotify.FSN_DELETE|fsnotify.FSN_RENAME|fsnotify.FSN_CREATE); err != nil {
			logrus.Warningf("fsnotify.Watcher.Watch(%s): %s", filename, err)
		}
	}

	return reopenErr
}

// Reopen file
func (l *FileLogger) Reopen() error {
	l.Lock()
	defer l.Unlock()

	var newFd *os.File
	var err error

	if l.filename != "" {
		newFd, err = os.OpenFile(l.filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)

		if err != nil {
			return err
		}
	} else {
		newFd = nil
	}

	oldFd := l.fd
	l.fd = newFd

	var loggerOut io.Writer

	if l.fd != nil {
		loggerOut = l.fd
	} else {
		loggerOut = os.Stderr
	}
	logrus.SetOutput(loggerOut)

	if oldFd != nil {
		oldFd.Close()
	}

	return nil
}

// Filename returns current filename
func (l *FileLogger) Filename() string {
	l.RLock()
	l.RUnlock()
	return l.filename
}

// SetFile for default logger
func SetFile(filename string) error {
	return std.Open(filename)
}
