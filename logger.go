package ettcodesdk

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

// 400: "Bad request."
// 401: "The request requires an user authentication."
// 402: "Not enough money on the contract"
// 403: "The access is not allowed."
// 404: "Wrong pagination parameters."
// 405: "Method not allowed."
// 409: "Outdated dataHash, provided product variant(s) are not available anymore"
// 410: "Selected products not available anymore"
// 413: "Too many products selected, change agent limitation"
// 422: "Failed creating order"
// 500: "Internal server error"
var ErrorCodeWithMessage = map[int]string{
	400: "Bad request.",
	401: "The request requires an user authentication.",
	402: "Not enough money on the contract",
	403: "The access is not allowed.",
	404: "Wrong pagination parameters.",
	405: "Method not allowed.",
	409: "Outdated dataHash, provided product variant(s) are not available anymore",
	410: "Selected products not available anymore",
	413: "Too many products selected, change agent limitation",
	422: "Failed creating order",
	500: "Internal server error",
}

type FaasLogger struct {
	WarningLog *Logger
	InfoLog    *Logger
	ErrorLog   *Logger
}

type Logger struct {
	mu     sync.Mutex // ensures atomic writes; protects the following fields
	prefix string     // prefix on each line to identify the logger (but see Lmsgprefix)
	flag   int        // properties
	buf    []byte     // for accumulating text to write
}

var Mode = []string{"🔵INFO", "🟡WARNING", "🔴ERROR"}

func NewLoggerFunction(functionName string) *FaasLogger {
	return &FaasLogger{
		InfoLog:    NewLoggerFaas(fmt.Sprintf("%s --- 🔵INFO --- ", functionName), log.Ldate|log.Ltime|log.Lshortfile),
		WarningLog: NewLoggerFaas(fmt.Sprintf("%s --- 🟡WARNING --- ", functionName), log.Ldate|log.Ltime|log.Lshortfile),
		ErrorLog:   NewLoggerFaas(fmt.Sprintf("%s --- 🔴ERROR --- ", functionName), log.Ldate|log.Ltime|log.Lshortfile),
	}
}
func NewLoggerFaas(prefix string, flag int) *Logger {
	l := &Logger{prefix: prefix, flag: flag}
	return l
}

const (
	Ldate         = 1 << iota     // the date in the local time zone: 2009/01/23
	Ltime                         // the time in the local time zone: 01:23:23
	Lmicroseconds                 // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                     // full file name and line number: /a/b/c/d.go:23
	Lshortfile                    // final file name element and line number: d.go:23. overrides Llongfile
	LUTC                          // if Ldate or Ltime is set, use UTC rather than the local time zone
	Lmsgprefix                    // move the "prefix" from the beginning of the line to before the message
	LstdFlags     = Ldate | Ltime // initial values for the standard logger
)

// Println calls l.Output to print to the logger.
// Arguments are handled in the manner of fmt.Println.
func (l *Logger) Sprint(v ...interface{}) string {
	return l.output(2, fmt.Sprintln(v...))
}

func (l *Logger) output(calldepth int, s string) string {
	now := time.Now() // get this early.
	var file string
	var line int
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.flag&(Lshortfile|Llongfile) != 0 {
		// Release lock while getting caller info - it's expensive.
		l.mu.Unlock()
		var ok bool
		_, file, line, ok = runtime.Caller(calldepth)
		if !ok {
			file = "???"
			line = 0
		}
		l.mu.Lock()
	}
	l.buf = l.buf[:0]
	l.formatHeader(&l.buf, now, file, line)
	l.buf = append(l.buf, s...)
	return string(l.buf)
}

// formatHeader writes log header to buf in following order:
//   - l.prefix (if it's not blank and Lmsgprefix is unset),
//   - date and/or time (if corresponding flags are provided),
//   - file and line number (if corresponding flags are provided),
//   - l.prefix (if it's not blank and Lmsgprefix is set).
func (l *Logger) formatHeader(buf *[]byte, t time.Time, file string, line int) {
	if l.flag&Lmsgprefix == 0 {
		*buf = append(*buf, l.prefix...)
	}
	if l.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		if l.flag&LUTC != 0 {
			t = t.UTC()
		}
		if l.flag&Ldate != 0 {
			year, month, day := t.Date()
			itoa(buf, year, 4)
			*buf = append(*buf, '-')
			itoa(buf, int(month), 2)
			*buf = append(*buf, '-')
			itoa(buf, day, 2)
			*buf = append(*buf, ' ')
		}
		if l.flag&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			itoa(buf, hour, 2)
			*buf = append(*buf, ':')
			itoa(buf, min, 2)
			*buf = append(*buf, ':')
			itoa(buf, sec, 2)
			if l.flag&Lmicroseconds != 0 {
				*buf = append(*buf, '.')
				itoa(buf, t.Nanosecond()/1e3, 6)
			}
			*buf = append(*buf, ' ')
		}
	}
	if l.flag&(Lshortfile|Llongfile) != 0 {
		if l.flag&Lshortfile != 0 {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
		}
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
		*buf = append(*buf, ": "...)
	}
	if l.flag&Lmsgprefix != 0 {
		*buf = append(*buf, l.prefix...)
	}
}

// Cheap integer to fixed-width decimal ASCII. Give a negative width to avoid zero-padding.
func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}
