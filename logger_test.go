package gormlogruslogger

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestLogger(t *testing.T, slowLogThreshold time.Duration, opts ...Option) (logger.Interface, *test.Hook) {
	t.Helper()

	l, hook := test.NewNullLogger()
	l.SetLevel(logrus.DebugLevel)

	return NewGormLogrusLogger(logrus.NewEntry(l), slowLogThreshold, opts...), hook
}

func TestMessages(t *testing.T) {
	tests := []struct {
		name      string
		mode      logger.LogLevel
		log       func(logger.Interface)
		wantLevel logrus.Level
		wantMsg   string
	}{
		{
			name:      "info",
			mode:      logger.Info,
			log:       func(l logger.Interface) { l.Info(context.Background(), "replacing callback %s", "create") },
			wantLevel: logrus.InfoLevel,
			wantMsg:   "replacing callback create",
		},
		{
			name:      "warn",
			mode:      logger.Info,
			log:       func(l logger.Interface) { l.Warn(context.Background(), "warning %d", 1) },
			wantLevel: logrus.WarnLevel,
			wantMsg:   "warning 1",
		},
		{
			name:      "error",
			mode:      logger.Info,
			log:       func(l logger.Interface) { l.Error(context.Background(), "failed: %v", errors.New("boom")) },
			wantLevel: logrus.ErrorLevel,
			wantMsg:   "failed: boom",
		},
		{
			name: "info suppressed in warn mode",
			mode: logger.Warn,
			log:  func(l logger.Interface) { l.Info(context.Background(), "hidden") },
		},
		{
			name:      "warn in warn mode",
			mode:      logger.Warn,
			log:       func(l logger.Interface) { l.Warn(context.Background(), "shown") },
			wantLevel: logrus.WarnLevel,
			wantMsg:   "shown",
		},
		{
			name: "warn suppressed in error mode",
			mode: logger.Error,
			log:  func(l logger.Interface) { l.Warn(context.Background(), "hidden") },
		},
		{
			name: "error suppressed in silent mode",
			mode: logger.Silent,
			log:  func(l logger.Interface) { l.Error(context.Background(), "hidden") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, hook := newTestLogger(t, time.Second)
			tt.log(l.LogMode(tt.mode))

			if tt.wantMsg == "" {
				if n := len(hook.AllEntries()); n != 0 {
					t.Fatalf("expected no log entries, got %d", n)
				}
				return
			}

			e := hook.LastEntry()
			if e == nil {
				t.Fatal("expected a log entry, got none")
			}
			if e.Level != tt.wantLevel {
				t.Errorf("level = %v, want %v", e.Level, tt.wantLevel)
			}
			if e.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", e.Message, tt.wantMsg)
			}
		})
	}
}

func TestLogModeReturnsCopy(t *testing.T) {
	l, hook := newTestLogger(t, time.Second)

	_ = l.LogMode(logger.Silent)
	l.Info(context.Background(), "still logged")

	if n := len(hook.AllEntries()); n != 1 {
		t.Fatalf("expected the original logger to keep logging, got %d entries", n)
	}
}

func TestTrace(t *testing.T) {
	errQuery := errors.New("syntax error")

	tests := []struct {
		name      string
		mode      logger.LogLevel
		threshold time.Duration
		elapsed   time.Duration
		err       error
		wantLevel logrus.Level
		wantError bool
		wantNone  bool
	}{
		{name: "regular query", mode: logger.Info, threshold: time.Second, wantLevel: logrus.DebugLevel},
		{name: "failed query", mode: logger.Info, threshold: time.Second, err: errQuery, wantLevel: logrus.WarnLevel, wantError: true},
		{name: "wrapped record not found", mode: logger.Info, threshold: time.Second, err: fmt.Errorf("lookup: %w", gorm.ErrRecordNotFound), wantNone: true},
		{name: "slow query", mode: logger.Info, threshold: time.Millisecond, elapsed: time.Second, wantLevel: logrus.WarnLevel},
		{name: "zero threshold disables slow log", mode: logger.Info, elapsed: time.Second, wantLevel: logrus.DebugLevel},
		{name: "regular query suppressed in warn mode", mode: logger.Warn, threshold: time.Second, wantNone: true},
		{name: "slow query in warn mode", mode: logger.Warn, threshold: time.Millisecond, elapsed: time.Second, wantLevel: logrus.WarnLevel},
		{name: "slow query suppressed in error mode", mode: logger.Error, threshold: time.Millisecond, elapsed: time.Second, wantNone: true},
		{name: "failed query in error mode", mode: logger.Error, threshold: time.Second, err: errQuery, wantLevel: logrus.WarnLevel, wantError: true},
		{name: "failed query suppressed in silent mode", mode: logger.Silent, threshold: time.Second, err: errQuery, wantNone: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, hook := newTestLogger(t, tt.threshold)

			called := false
			fc := func() (string, int64) {
				called = true
				return "SELECT * FROM users", 3
			}

			l.LogMode(tt.mode).Trace(context.Background(), time.Now().Add(-tt.elapsed), fc, tt.err)

			if tt.wantNone {
				if n := len(hook.AllEntries()); n != 0 {
					t.Fatalf("expected no log entries, got %d", n)
				}
				if called {
					t.Error("expected the SQL not to be rendered for a suppressed entry")
				}
				return
			}

			e := hook.LastEntry()
			if e == nil {
				t.Fatal("expected a log entry, got none")
			}
			if e.Level != tt.wantLevel {
				t.Errorf("level = %v, want %v", e.Level, tt.wantLevel)
			}
			if e.Message != "SELECT * FROM users" {
				t.Errorf("message = %q, want the SQL statement", e.Message)
			}
			if e.Data["rows"] != int64(3) {
				t.Errorf("rows = %v, want 3", e.Data["rows"])
			}
			if _, ok := e.Data["duration"].(float64); !ok {
				t.Errorf("duration = %v, want a float64", e.Data["duration"])
			}
			if _, ok := e.Data["error"]; ok != tt.wantError {
				t.Errorf("error field present = %v, want %v", ok, tt.wantError)
			}
		})
	}
}

func TestParamsFilterWithGorm(t *testing.T) {
	type user struct {
		ID   uint
		Name string
	}

	tests := []struct {
		name      string
		opts      []Option
		wantValue bool
	}{
		{name: "values are rendered by default", wantValue: true},
		{name: "parameterized queries hide values", opts: []Option{WithParameterizedQueries()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, hook := newTestLogger(t, time.Minute, tt.opts...)

			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: l})
			if err != nil {
				t.Fatalf("opening database: %v", err)
			}
			if err := db.AutoMigrate(&user{}); err != nil {
				t.Fatalf("migrating: %v", err)
			}

			hook.Reset()

			var users []user
			if err := db.Where("name = ?", "secret-name").Find(&users).Error; err != nil {
				t.Fatalf("querying: %v", err)
			}

			e := hook.LastEntry()
			if e == nil {
				t.Fatal("expected the query to be logged")
			}
			if got := strings.Contains(e.Message, "secret-name"); got != tt.wantValue {
				t.Errorf("logged SQL %q contains the value = %v, want %v", e.Message, got, tt.wantValue)
			}
		})
	}
}
