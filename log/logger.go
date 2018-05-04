package log

import (
	"log"
	"os"
	"bytes"
	"strconv"
	"fmt"
	"sync"
	"time"
)

const errorKey = "LOG15_ERROR"
const floatFormat = 'f'

// A Record is what a Logger asks its handler to write
type Record struct {
	Time     time.Time
	//Lvl      Lvl
	Msg      string
	Ctx      []interface{}
	//Call     stack.Call
	KeyNames RecordKeyNames
}

type RecordKeyNames struct {
	Time string
	Msg  string
	Lvl  string
}

type Logger interface {
	Trace(msg string, ctx ...interface{})
	Debug(msg string, ctx ...interface{})
	Info(msg string, ctx ...interface{})
	Warn(msg string, ctx ...interface{})
	Error(msg string, ctx ...interface{})
	Crit(msg string, ctx ...interface{})
}

type logger struct {
	trace *log.Logger
	debug *log.Logger
	info *log.Logger
	warn *log.Logger
	error *log.Logger
	crit *log.Logger
}

type Ctx map[string]interface{}

func (c Ctx) toArray() []interface{} {
	arr := make([]interface{}, len(c)*2)

	i := 0
	for k, v := range c {
		arr[i] = k
		arr[i+1] = v
		i += 2
	}

	return arr
}

func normalize(ctx []interface{}) []interface{} {
	// if the caller passed a Ctx object, then expand it
	if len(ctx) == 1 {
		if ctxMap, ok := ctx[0].(Ctx); ok {
			ctx = ctxMap.toArray()
		}
	}

	// expected to be even, as we are expecting key value pairs
	// in case of missing pair, log with sufficient information
	// indicating the miss
	if len(ctx) % 2 != 0 {
		ctx = append(ctx, nil, errorKey, "nil added to Normalize Odd number of arguments")
	}

	return ctx
}

func NewContext(prefix []interface{}, suffix []interface{}) []interface{} {
	normalizedSuffix := normalize(suffix)
	newCtx := make([]interface{}, len(prefix)+len(normalizedSuffix))
	n := copy(newCtx, prefix)
	copy(newCtx[n:], normalizedSuffix)
	return newCtx
}

func New() Logger {
	handler := os.Stdout
	logger := &logger{
		trace: log.New(handler, "TRACE ", log.Ldate|log.Ltime),
		debug: log.New(handler, "DEBUG ", log.Ldate|log.Ltime),
		info: log.New(handler, "INFO ", log.Ldate|log.Ltime),
		warn: log.New(handler, "WARN ", log.Ldate|log.Ltime),
		error: log.New(handler, "ERROR ", log.Ldate|log.Ltime),
		crit: log.New(handler, "CRIT ", log.Ldate|log.Ltime),
	}
	return logger
}

func (l *logger) Trace(msg string, ctx ...interface{}) {
	record := &Record {Msg: msg, Ctx: normalize(ctx)}
	l.trace.Println(msg, TerminalFormat(record))
}

func (l *logger) Debug(msg string, ctx ...interface{}) {
	record := &Record {Msg: msg, Ctx: normalize(ctx)}
	l.debug.Println(msg, TerminalFormat(record))
}

func (l *logger) Info(msg string, ctx ...interface{}) {
	record := &Record {Msg: msg, Ctx: normalize(ctx)}
	l.info.Println(msg, TerminalFormat(record))
}

func (l *logger) Warn(msg string, ctx ...interface{}) {
	record := &Record {Msg: msg, Ctx: normalize(ctx)}
	l.warn.Println(msg, TerminalFormat(record))
}

func (l *logger) Error(msg string, ctx ...interface{}) {
	record := &Record {Msg: msg, Ctx: normalize(ctx)}
	l.error.Println(msg, TerminalFormat(record))
}

func (l *logger) Crit(msg string, ctx ...interface{}) {
	record := &Record {Msg: msg, Ctx: normalize(ctx)}
	l.crit.Println(msg, TerminalFormat(record))
}

var stringBufPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

func escapeString(s string) string {
	needsQuotes := false
	needsEscape := false
	for _, r := range s {
		if r <= ' ' || r == '=' || r == '"' {
			needsQuotes = true
		}
		if r == '\\' || r == '"' || r == '\n' || r == '\r' || r == '\t' {
			needsEscape = true
		}
	}
	if !needsEscape && !needsQuotes {
		return s
	}
	e := stringBufPool.Get().(*bytes.Buffer)
	e.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			e.WriteByte('\\')
			e.WriteByte(byte(r))
		case '\n':
			e.WriteString("\\n")
		case '\r':
			e.WriteString("\\r")
		case '\t':
			e.WriteString("\\t")
		default:
			e.WriteRune(r)
		}
	}
	e.WriteByte('"')
	var ret string
	if needsQuotes {
		ret = e.String()
	} else {
		ret = string(e.Bytes()[1 : e.Len()-1])
	}
	e.Reset()
	stringBufPool.Put(e)
	return ret
}

func formatLogfmtValue(value interface{}) string {
	if value == nil {
		return "nil"
	}

	switch v := value.(type) {
	case bool:
		return strconv.FormatBool(v)
	case float32:
		return strconv.FormatFloat(float64(v), floatFormat, 3, 64)
	case float64:
		return strconv.FormatFloat(v, floatFormat, 3, 64)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", value)
	case string:
		return escapeString(v)
	default:
		return escapeString(fmt.Sprintf("%+v", value))
	}
}

func TerminalFormat(r *Record) string {
	ctx := r.Ctx
	if len(ctx) == 1 {
		k, _ := ctx[0].(string)
		return k
	}
	buf := &bytes.Buffer{}
	for i:=0; i < len(ctx); i += 2 {
		if i != 0 {
			buf.WriteByte(' ')
		}

		k, ok := ctx[i].(string)
		v := formatLogfmtValue(ctx[i+1])
		if !ok {
			k, v = errorKey, formatLogfmtValue(k)
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(v)
	}
	return buf.String()
}