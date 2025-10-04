package log2parquet

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"time"

	pq "github.com/parquet-go/parquet-go"
)

var (
	ErrNoTimestamp      error = errors.New("timestamp missing")
	ErrInvalidTimestamp error = errors.New("invalid timestamp")

	ErrNoSeverity      error = errors.New("severity missing")
	ErrInvalidSeverity error = errors.New("invalid severity")

	ErrNoBody      error = errors.New("body missing")
	ErrInvalidBody error = errors.New("invalid body")
)

type Severity uint8

const (
	SeverityUnspecified Severity = 0

	SeverityTrace Severity = 1
	SeverityDebug Severity = 5
	SeverityInfo  Severity = 9
	SeverityWarn  Severity = 13
	SeverityError Severity = 17
	SeverityFatal Severity = 21
)

type SeverityText string

const (
	SeverityTextUnspecified SeverityText = "unspecified"

	SeverityTextTrace SeverityText = "trace"
	SeverityTextDebug SeverityText = "debug"
	SeverityTextInfo  SeverityText = "info"
	SeverityTextWarn  SeverityText = "warn"
	SeverityTextError SeverityText = "error"
	SeverityTextFatal SeverityText = "fatal"
)

var severityToText = map[Severity]SeverityText{
	SeverityUnspecified: SeverityTextUnspecified,
	SeverityTrace:       SeverityTextTrace,
	SeverityDebug:       SeverityTextDebug,
	SeverityInfo:        SeverityTextInfo,
	SeverityWarn:        SeverityTextWarn,
	SeverityError:       SeverityTextError,
	SeverityFatal:       SeverityTextFatal,
}

func (s Severity) ToText() SeverityText {
	if t, ok := severityToText[s]; ok {
		return t
	}
	return SeverityTextUnspecified
}

var textToSeverity = map[SeverityText]Severity{
	SeverityTextUnspecified: SeverityUnspecified,
	SeverityTextTrace:       SeverityTrace,
	SeverityTextDebug:       SeverityDebug,
	SeverityTextInfo:        SeverityInfo,
	SeverityTextWarn:        SeverityWarn,
	SeverityTextError:       SeverityError,
	SeverityTextFatal:       SeverityFatal,
}

func SeverityFromText(t SeverityText) Severity {
	s, ok := textToSeverity[t]
	switch ok {
	case true:
		return s
	default:
		return SeverityUnspecified
	}
}

type GenericLog struct {
	Timestamp time.Time
	Severity  int32
	Body      string
	Json      string
}

type GenericLogBuilder struct {
	TimestampName string
	SeverityName  string
	BodyName      string
}

var GenericLogBuilderDefault GenericLogBuilder = GenericLogBuilder{
	TimestampName: "timestamp",
	SeverityName:  "severity",
	BodyName:      "body",
}

func AnyToType[T any](val any) (t T, b bool) {
	switch v := val.(type) {
	case T:
		return v, true
	default:
		return t, false
	}
}

//nolint:varnamelen
func (b GenericLogBuilder) MapToTime(m map[string]any) (t time.Time, e error) {
	atime, ok := m[b.TimestampName]
	if !ok {
		return t, ErrNoTimestamp
	}

	stime, ok := AnyToType[string](atime)
	if !ok {
		return t, ErrInvalidTimestamp
	}

	timestamp, e := time.Parse(time.RFC3339Nano, stime)
	if nil != e {
		return t, fmt.Errorf("%w: %w", ErrInvalidTimestamp, e)
	}

	return timestamp, nil
}

//nolint:varnamelen
func (b GenericLogBuilder) MapToSeverity(m map[string]any) (Severity, error) {
	aseverity, ok := m[b.SeverityName]
	if !ok {
		return SeverityUnspecified, ErrNoSeverity
	}

	tseverity, ok := AnyToType[string](aseverity)
	if !ok {
		return SeverityUnspecified, ErrInvalidSeverity
	}

	return SeverityFromText(SeverityText(tseverity)), nil
}

//nolint:varnamelen
func (b GenericLogBuilder) MapToBody(m map[string]any) (string, error) {
	abody, ok := m[b.BodyName]
	if !ok {
		return "", ErrNoBody
	}

	sbody, ok := AnyToType[string](abody)
	if !ok {
		return "", ErrInvalidBody
	}

	return sbody, nil
}

//nolint:varnamelen
func (b GenericLogBuilder) ReaderToGenericLogs(rdr io.Reader) iter.Seq2[GenericLog, error] {
	return func(yield func(GenericLog, error) bool) {
		var s *bufio.Scanner = bufio.NewScanner(rdr)
		var buf map[string]any
		var row GenericLog
		for s.Scan() {
			var line []byte = s.Bytes()
			clear(buf)
			e := json.Unmarshal(line, &buf)
			if nil != e {
				yield(row, e)
				return
			}

			timestamp, e := b.MapToTime(buf)
			if nil != e {
				yield(row, e)
				return
			}
			row.Timestamp = timestamp

			severity, e := b.MapToSeverity(buf)
			if nil != e {
				yield(row, e)
				return
			}
			row.Severity = int32(severity)

			sbody, e := b.MapToBody(buf)
			if nil != e {
				yield(row, e)
				return
			}

			row.Body = sbody

			delete(buf, b.TimestampName)
			delete(buf, b.SeverityName)
			delete(buf, b.BodyName)

			encoded, e := json.Marshal(buf)
			if nil != e {
				yield(row, e)
				return
			}

			row.Json = string(encoded)
			if !yield(row, nil) {
				return
			}
		}
	}
}

func (b GenericLogBuilder) StdinToGenericLogs() iter.Seq2[GenericLog, error] {
	return b.ReaderToGenericLogs(os.Stdin)
}

type GenericLogWriter struct {
	*pq.GenericWriter[GenericLog]
}

func (w GenericLogWriter) WriteAll(logs iter.Seq2[GenericLog, error]) error {
	for row, e := range logs {
		if nil != e {
			return e
		}

		_, e := w.GenericWriter.Write([]GenericLog{row})
		if nil != e {
			return e
		}
	}

	return w.GenericWriter.Flush()
}

func (w GenericLogWriter) Close() error { return w.GenericWriter.Close() }

type GenericWriterBuilder struct {
	Writer  io.Writer
	Options []pq.WriterOption
}

func (b GenericWriterBuilder) ToWriter() GenericLogWriter {
	wtr := pq.NewGenericWriter[GenericLog](b.Writer, b.Options...)
	return GenericLogWriter{GenericWriter: wtr}
}

var GenericWriterBuilderDefault GenericWriterBuilder = GenericWriterBuilder{
	Writer:  os.Stdout,
	Options: nil,
}

func StdinToGenericLogsToParquetToStdoutDefault() error {
	var bldr GenericLogBuilder = GenericLogBuilderDefault
	var logs iter.Seq2[GenericLog, error] = bldr.StdinToGenericLogs()
	var wtr GenericLogWriter = GenericWriterBuilderDefault.ToWriter()
	e := wtr.WriteAll(logs)
	return errors.Join(e, wtr.Close())
}
