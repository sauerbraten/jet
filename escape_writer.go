package jet

import "io"

type EscapeFunc func(io.Writer, []byte)

func NoopEscape(w io.Writer, b []byte) {
	w.Write(b)
}

func unsafePrinter(w io.Writer, b []byte) {
	w.Write(b)
}

type EscapeWriter struct {
	w      io.Writer
	escape EscapeFunc
}

func (ew *EscapeWriter) Write(b []byte) (n int, err error) {
	ew.escape(ew.w, b)
	return 0, nil
}
