package testutil

import (
	"encoding/hex"
	"io"
	"net/url"
	"testing"

	"go.uber.org/zap"
)

func NewLogger(t *testing.T) *zap.SugaredLogger {
	return NewLoggerWithReadWriter(t, newNullReadWriter())
}

// newNullReadWriter is for zap logging to /dev/null.
func newNullReadWriter() *nullReadWriter {
	return &nullReadWriter{Writer: io.Discard}
}

type nullReadWriter struct{ io.Writer }

func (r nullReadWriter) Read(b []byte) (int, error) {
	return 0, nil
}

// NewLoggerWithReadWriter provides a zap logger from an io.ReadWriter.
//
// Using a ReadWriter is easier than having to deal with files on disk.
func NewLoggerWithReadWriter(t *testing.T, rw io.ReadWriter) *zap.SugaredLogger {
	// Zap doesn't provide a mechanism for using a bare io.Writer as a log. :(
	// But they do allow the registration of a scheme and factory pair. With
	// the generation of a unique scheme, a test log can be built that uses an
	// io.Writer. The scheme has to be unique, because zap won't allow the
	// registration of a scheme twice.
	scheme := TestScheme(t)
	factory := func(u *url.URL) (zap.Sink, error) { return newTestZapSink(rw), nil }
	if err := zap.RegisterSink(scheme, factory); err != nil {
		t.Fatalf("registering zap scheme %q: %s", scheme, err)
	}
	cfg := zap.NewDevelopmentConfig()
	cfg.OutputPaths = []string{scheme + "://" + t.Name()}
	base, err := cfg.Build()
	if err != nil {
		t.Fatalf("building zap logger: %s", err)
	}
	return base.Sugar()
}

// TestScheme generates a scheme that's unique to the test.
//
// It relies on testing.T.Name providing a unique name (which it should).
func TestScheme(t *testing.T) string {
	// schemes must start with [a-zA-Z]
	return "t" + hex.EncodeToString([]byte(t.Name()))
}

// testZapSink adapts an io.Writer to function as a zap.Sink.
//
// Using a io.Writer allows us to skip needing to write logs to disk, which is
// handy for testing.
type testZapSink struct {
	io.Writer
}

func newTestZapSink(w io.Writer) *testZapSink {
	return &testZapSink{
		Writer: w,
	}
}

// Write implements zap.Sink
func (s *testZapSink) Write(p []byte) (n int, err error) {
	return s.Writer.Write(p)
}

// Sync implements zap.Sink
func (s *testZapSink) Sync() error {
	return nil
}

// Close implements zap.Sink
func (s *testZapSink) Close() error {
	return nil
}
