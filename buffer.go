package lager

// Low-level code for composing a log line.

import(
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"sync"
	"time"
)


// TYPES //

// An unshared, temporary structure for efficiently logging one line.
type buffer struct {
	scratch [16*1024]byte   // Space so we can allocate memory only rarely.
	buf     []byte          // Bytes not yet written (a slice into above).
	w       io.Writer       // Usually os.Stdout, else os.Stderr.
	delim   string          // Delimiter to go before next value.
	locked  bool            // Whether we had to lock outMu.
}


// GLOBALS //

// Minimize how many of these must be allocated:
var bufPool = sync.Pool{New: func() interface{} {
	b := new(buffer)
	b.buf = b.scratch[0:0]
	return b
}}

// A lock in case a log line is too large to buffer.
var outMu sync.Mutex

// The (JSON) delimiter between values:
const comma = ", "


// FUNCS //

var noEsc [256]bool
var hexDigits = "0123456789abcdef"

func init() {
	for c := ' '; c < 256; c++ {
		noEsc[c] = true
	}
	noEsc['"'] = false
	noEsc['\\'] = false
}

// Called when we need to flush early, to prevent interleaved log lines.
func (b *buffer) lock() {
	if ! b.locked {
		outMu.Lock()
		b.locked = true
	}
	if 0 < len(b.buf) {
		b.w.Write(b.buf)
		b.buf = b.scratch[0:0]
	}
}

// Called when finished composing a log line.
func (b *buffer) unlock() {
	if 0 < len(b.buf) {
		b.w.Write(b.buf)
		b.buf = b.scratch[0:0]
	}
	if b.locked {
		b.locked = false
		outMu.Unlock()
	}
}

// Append a slice of bytes to the log line.
func (b *buffer) writeBytes(s []byte) {
	if cap(b.buf) < len(b.buf) + len(s) {
		b.lock()    // Can't fit line in buffer; lock output mutex and flush.
	}
	if cap(b.buf) < len(s) {
		b.w.Write(s)    // Next chunk won't fit in buffer, just write it.
	} else {
		b.buf = append(b.buf, s...)
	}
}

// Append strings to the log line.
func (b *buffer) write(strs ...string) {
	for _, s := range strs {
		if cap(b.buf) < len(b.buf) + len(s) {
			b.lock()
		}
		if cap(b.buf) < len(s) {
			io.WriteString(b.w, s)
		} else {
			was := len(b.buf)
			will := was + len(s)
			b.buf = b.buf[0:will]
			for i := 0; i < len(s); i++ {
				b.buf[was+i] = s[i]
			}
		}
	}
}

// Append a quoted (JSON) string to the log lien.
func (b *buffer) quote(s string) {
	b.write(b.delim, `"`)
	b.escape(s)
	b.write(`"`)
	b.delim = comma
}

// Append a quoted (JSON) string (from a byte slice) to the log line.
func (b *buffer) quoteBytes(s []byte) {
	b.write(b.delim, `"`)
	b.escapeBytes(s)
	b.write(`"`)
}

// Append an escaped string as part of a quoted JSON string.
func (b *buffer) escape(s string) {
	beg := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if noEsc[c] {
			continue
		}
		next := ""
		switch c {
		case '"':   next = "\\\""
		case '\\':  next = "\\\\"
		case '\b':  next = "\\b"
		case '\f':  next = "\\f"
		case '\n':  next = "\\n"
		case '\r':  next = "\\r"
		case '\t':  next = "\\t"
		default:
			buf := []byte("\\u0000")
			for o := 5; 1 < o; o-- {
				h := c & 0xF
				buf[o] = hexDigits[h]
				c >>= 4
			}
			next = string(buf)
		}
		b.write(s[beg:i])
		b.write(next)
		beg = i+1
	}
	b.write(s[beg:])
}

// Append an escaped string (from a byte slice), part of a quoted JSON string.
func (b *buffer) escapeBytes(s []byte) {
	beg := 0
	for i, c := range s {
		if noEsc[c] {
			continue
		}
		next := ""
		switch c {
		case '"':   next = "\\\""
		case '\\':  next = "\\\\"
		case '\b':  next = "\\b"
		case '\f':  next = "\\f"
		case '\n':  next = "\\n"
		case '\r':  next = "\\r"
		case '\t':  next = "\\t"
		default:
			buf := []byte("\\u0000")
			for o := 5; 1 < o; o-- {
				h := c & 0xF
				buf[o] = hexDigits[h]
				c >>= 4
			}
			next = string(buf)
		}
		b.writeBytes(s[beg:i])
		b.write(next)
		beg = i+1
	}
	b.writeBytes(s[beg:])
}

// Append a 2-digit value to the buffer (with leading '0').
func (b *buffer) int2(val int) {
//  if cap(b.buf) < len(b.buf) + 2 {
//      b.lock()
//  }
	l := len(b.buf)
	b.buf = b.buf[0:2+l]
	b.buf[l] = '0' + byte(val/10)
	b.buf[l+1] = '0' + byte(val%10)
}

// Append a decimal value of specified length with leading '0's.
func (b *buffer) int(val int, digits int) {
//  if cap(b.buf) < len(b.buf) + digits {
//      b.lock()
//  }
	bef := len(b.buf)
	b.buf = strconv.AppendInt(b.buf, int64(val), 10)
	aft := len(b.buf)
	l := aft-bef
	// Prepend leading '0's to get desired length:
	if l < digits {
		b.buf = b.buf[0:bef+digits]
		copy(b.buf[bef+digits-l:bef+digits], b.buf[bef:aft])
		for i := bef; i < bef+digits-l; i++ {
			b.buf[i] = '0'
		}
	}
}

// Append a quoted UTC timestamp to the log line.
func (b *buffer) timestamp() {
	if cap(b.buf) < len(b.buf) + 22 {
		b.lock()
	}
	now := time.Now().In(time.UTC)
	b.write(`"`)
	yr, mo, day := now.Date()
	b.buf = strconv.AppendInt(b.buf, int64(yr), 10)
	b.write("-")
	b.int2(int(mo))
	b.write("-")
	b.int2(day)
	b.write(" ")
	b.int2(now.Hour())
	b.write(":")
	b.int2(now.Minute())
	b.write(":")
	b.int2(now.Second())
	b.write(".")
	b.int(now.Nanosecond()/1000000, 3)
	b.write(`Z"`)
	b.delim = comma
}

// Begin appending a nested data structure to the log line.
func (b *buffer) open(punct string) {
	b.write(b.delim, punct)
	b.delim = ""
}

// Append the key/value separator ":" to the log line.
func (b *buffer) colon() {
	b.write(":")
	b.delim = ""
}

// End appending a nested data structure to the log line.
func (b *buffer) close(punct string) {
	b.write(punct)
	b.delim = comma
}

// Append a single key/value pair:
func (b *buffer) pair(k string, v interface{}) {
	b.quote(k)
	b.colon()
	b.scalar(v)
}

// Append the key/value pairs from AMap:
func (b *buffer) pairs(m AMap) {
	if nil != m {
		for i, k := range m.keys {
			b.pair(k, m.vals[i])
		}
	}
}

// Append a JSON-encoded scalar value to the log line.
func (b *buffer) scalar(s interface{}) {
	switch v := s.(type) {
	case AMap: if nil == v || 0 == len(v.keys) { return }
	}
	b.write(b.delim)
	b.delim = ""
	if cap(b.buf) < len(b.buf) + 64 {
		b.lock()    // Leave room for strconv.AppendFloat() or similar
	}
	switch v := s.(type) {
	case nil:       b.write("null")
	case string:    b.quote(v)
	case []byte:    b.quoteBytes(v)
	case int:       b.buf = strconv.AppendInt(b.buf, int64(v), 10)
	case int8:      b.buf = strconv.AppendInt(b.buf, int64(v), 10)
	case int16:     b.buf = strconv.AppendInt(b.buf, int64(v), 10)
	case int32:     b.buf = strconv.AppendInt(b.buf, int64(v), 10)
	case int64:     b.buf = strconv.AppendInt(b.buf, v, 10)
	case uint:      b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
	case uint8:     b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
	case uint16:    b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
	case uint32:    b.buf = strconv.AppendUint(b.buf, uint64(v), 10)
	case uint64:    b.buf = strconv.AppendUint(b.buf, v, 10)
	case float32:   b.buf = strconv.AppendFloat(b.buf, float64(v), 'g', -1, 32)
	case float64:   b.buf = strconv.AppendFloat(b.buf, v, 'g', -1, 64)
	case bool:
		if v {
			b.write("true")
		} else {
			b.write("false")
		}
	case []string:
		b.open("[")
		for _, s := range v { b.scalar(s) }
		b.close("]")
	case AList:
		b.open("[")
		for _, s := range v { b.scalar(s) }
		b.close("]")
	case RawMap:
		b.open("{")
		for i, elt := range v {
			if 0 == 1 & i {
				b.quote(S(elt))
				b.colon()
			} else {
				b.scalar(elt)
			}
		}
		if 1 == 1 & len(v) {
			b.scalar(nil)
		}
		b.close("}")
	case AMap:
		b.open("{")
		b.pairs(v)
		b.close("}")
	case map[string]interface{}:
		keys := make([]string, len(v))
		i := 0
		for k, _ := range v {
			keys[i] = k
			i++
		}
		sort.Strings(keys)
		b.open("{")
		for _, k := range keys {
			b.pair(k, v[k])
		}
		b.close("}")
	case error:
		b.quote(v.Error())
	default:
		buf, err := json.Marshal(v)
		if nil != err {
			b.quote("! " + err.Error())
		} else {
			b.writeBytes(buf)
		}
	}
	b.delim = comma
}
