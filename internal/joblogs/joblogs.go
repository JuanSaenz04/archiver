package joblogs

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	MaxEntries   = 2000
	MaxLineBytes = 4096
	SafetyTTL    = 24 * time.Hour
	SuccessTTL   = time.Hour
	batchSize    = 100
	queueSize    = 512
)

type Entry struct {
	ID     string `json:"id"`
	Time   string `json:"time"`
	Source string `json:"source"`
	Text   string `json:"text"`
}

type Store struct{ client *redis.Client }

func NewStore(client *redis.Client) *Store {
	return &Store{client: client.WithTimeout(500 * time.Millisecond)}
}
func Key(jobID string) string { return "job:" + jobID + ":logs" }

func (s *Store) append(ctx context.Context, jobID string, entries []Entry) error {
	_, err := s.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		for _, e := range entries {
			p.XAdd(ctx, &redis.XAddArgs{Stream: Key(jobID), MaxLen: MaxEntries, Values: map[string]any{"time": e.Time, "source": e.Source, "text": e.Text}})
		}
		p.Expire(ctx, Key(jobID), SafetyTTL)
		return nil
	})
	return err
}

// Read returns a bounded batch strictly after the cursor, or the recent tail
// when no cursor is supplied. Readers keep their own cursors.
func (s *Store) Read(ctx context.Context, jobID, cursor string) ([]Entry, error) {
	var messages []redis.XMessage
	var err error
	if cursor == "" {
		messages, err = s.client.XRevRangeN(ctx, Key(jobID), "+", "-", 500).Result()
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	} else {
		messages, err = s.client.XRangeN(ctx, Key(jobID), "("+cursor, "+", batchSize).Result()
	}
	entries := make([]Entry, 0, len(messages))
	for _, m := range messages {
		entries = append(entries, Entry{ID: m.ID, Time: fmt.Sprint(m.Values["time"]), Source: fmt.Sprint(m.Values["source"]), Text: fmt.Sprint(m.Values["text"])})
	}
	return entries, err
}

func (s *Store) Status(ctx context.Context, jobID string) (string, error) {
	return s.client.HGet(ctx, "job:"+jobID, "status").Result()
}

func (s *Store) Available(ctx context.Context, jobID string) (bool, error) {
	n, err := s.client.Exists(ctx, Key(jobID)).Result()
	return n > 0, err
}

func (s *Store) HistoryMissing(ctx context.Context, jobID, cursor string) (bool, error) {
	if cursor == "" || cursor == "0-0" {
		return false, nil
	}
	first, err := s.client.XRangeN(ctx, Key(jobID), "-", "+", 1).Result()
	if err != nil || len(first) == 0 {
		return false, err
	}
	a, b := strings.Split(cursor, "-"), strings.Split(first[0].ID, "-")
	for i := range 2 {
		if len(a[i]) != len(b[i]) {
			return len(a[i]) < len(b[i]), nil
		}
		if a[i] != b[i] {
			return a[i] < b[i], nil
		}
	}
	return false, nil
}

type Session struct {
	ctx            context.Context
	cancel         context.CancelFunc
	store          *Store
	jobID          string
	queue          chan Entry
	done           chan struct{}
	dropped        atomic.Uint64
	stdout, stderr *lineWriter
	once           sync.Once
}

func (s *Store) Start(jobID string) *Session {
	l := &Session{store: s, jobID: jobID, queue: make(chan Entry, queueSize), done: make(chan struct{})}
	l.ctx, l.cancel = context.WithCancel(context.Background())
	l.stdout = &lineWriter{session: l, source: "stdout"}
	l.stderr = &lineWriter{session: l, source: "stderr"}
	go l.publish()
	return l
}

func (s *Session) Stdout() io.Writer   { return s.stdout }
func (s *Session) Stderr() io.Writer   { return s.stderr }
func (s *Session) Message(text string) { s.enqueue("system", text) }

// FlushOutput emits trailing fragments after the process has stopped writing.
func (s *Session) FlushOutput() { s.stdout.finish(); s.stderr.finish() }

func (s *Session) enqueue(source, text string) {
	if len(text) > MaxLineBytes {
		text = text[:MaxLineBytes] + " [truncated]"
	}
	e := Entry{Time: time.Now().UTC().Format(time.RFC3339Nano), Source: source, Text: strings.ToValidUTF8(text, "�")}
	select {
	case s.queue <- e:
	default:
		s.dropped.Add(1)
	}
}

// Close must be called after output writers and lifecycle producers have stopped.
// Publishing uses independent, bounded contexts so cancellation still flushes logs.
func (s *Session) Close(failed bool) {
	s.once.Do(func() {
		timer := time.AfterFunc(2*time.Second, s.cancel)
		defer timer.Stop()
		defer s.cancel()
		s.FlushOutput()
		close(s.queue)
		<-s.done
		ttl := SuccessTTL
		if failed {
			ttl = SafetyTTL
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.store.client.Expire(ctx, Key(s.jobID), ttl).Err()
	})
}

func (s *Session) publish() {
	defer close(s.done)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	batch := make([]Entry, 0, batchSize)
	flush := func() {
		n := s.dropped.Swap(0)
		lost := uint64(len(batch)) + n
		if n > 0 {
			batch = append(batch, Entry{Time: time.Now().UTC().Format(time.RFC3339Nano), Source: "system", Text: fmt.Sprintf("%d log lines omitted", n)})
		}
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(s.ctx, 500*time.Millisecond)
		if err := s.store.append(ctx, s.jobID, batch); err != nil {
			s.dropped.Add(lost)
		}
		cancel()
		batch = batch[:0]
	}
	for {
		if s.ctx.Err() != nil {
			return
		}
		select {
		case <-s.ctx.Done():
			return
		case entry, ok := <-s.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, entry)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

type lineWriter struct {
	mu        sync.Mutex
	session   *Session
	source    string
	partial   []byte
	truncated bool
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, b := range p {
		if b == '\n' {
			w.emit()
			continue
		}
		if len(w.partial) < MaxLineBytes {
			w.partial = append(w.partial, b)
		} else {
			w.truncated = true
		}
	}
	return len(p), nil
}

func (w *lineWriter) emit() {
	text := strings.TrimSuffix(string(w.partial), "\r")
	if w.truncated {
		text += " [truncated]"
	}
	w.session.enqueue(w.source, text)
	w.partial = w.partial[:0]
	w.truncated = false
}

func (w *lineWriter) finish() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.partial) > 0 || w.truncated {
		w.emit()
	}
}
