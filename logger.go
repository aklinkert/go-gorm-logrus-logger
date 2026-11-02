package gormlogruslogger

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Option configures the logger returned by NewGormLogrusLogger.
type Option func(*gormLogger)

// WithParameterizedQueries makes the logger log SQL statements with
// placeholders instead of the bound parameter values, keeping potentially
// sensitive values out of the logs.
func WithParameterizedQueries() Option {
	return func(g *gormLogger) {
		g.parameterizedQueries = true
	}
}

// NewGormLogrusLogger returns a new gorm/logger.Interface compliant logger that uses logrus for the actual logging.
//
// Queries taking longer than slowLogThreshold are logged as warnings, a zero
// threshold disables slow query logging. All other queries are logged at debug
// level, so the level of the logrus logger decides whether they show up.
func NewGormLogrusLogger(entry *logrus.Entry, slowLogThreshold time.Duration, opts ...Option) logger.Interface {
	g := &gormLogger{
		entry:            entry,
		slowLogThreshold: slowLogThreshold,
		level:            logger.Info,
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

var (
	_ logger.Interface  = &gormLogger{}
	_ gorm.ParamsFilter = &gormLogger{}
)

type gormLogger struct {
	entry                *logrus.Entry
	slowLogThreshold     time.Duration
	parameterizedQueries bool
	level                logger.LogLevel
}

// LogMode returns a copy of the logger that only logs messages of the given
// gorm log level or above, e.g. logger.Silent disables logging entirely.
func (g *gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	c := *g
	c.level = level

	return &c
}

func (g *gormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if g.level >= logger.Info {
		g.entry.WithContext(ctx).Infof(msg, data...)
	}
}

func (g *gormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if g.level >= logger.Warn {
		g.entry.WithContext(ctx).Warnf(msg, data...)
	}
}

func (g *gormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if g.level >= logger.Error {
		g.entry.WithContext(ctx).Errorf(msg, data...)
	}
}

func (g *gormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if g.level <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	duration := float64(elapsed.Nanoseconds()) / 1e6
	entry := g.entry.WithContext(ctx)

	switch {
	case err != nil:
		// record not found is an expected error and thus not logged
		if errors.Is(err, gorm.ErrRecordNotFound) || g.level < logger.Error {
			return
		}

		sql, rows := fc()
		entry.WithFields(logrus.Fields{
			"error":    err,
			"rows":     rows,
			"duration": duration,
		}).Warn(sql)

	case g.slowLogThreshold > 0 && elapsed > g.slowLogThreshold:
		if g.level < logger.Warn {
			return
		}

		sql, rows := fc()
		entry.WithFields(logrus.Fields{
			"rows":     rows,
			"duration": duration,
		}).Warn(sql)

	default:
		if g.level < logger.Info {
			return
		}

		sql, rows := fc()
		entry.WithFields(logrus.Fields{
			"rows":     rows,
			"duration": duration,
		}).Debug(sql)
	}
}

// ParamsFilter implements gorm.ParamsFilter. It drops the bound parameters
// when the logger was created WithParameterizedQueries, so gorm renders the
// logged SQL with placeholders.
func (g *gormLogger) ParamsFilter(_ context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if g.parameterizedQueries {
		return sql, nil
	}

	return sql, params
}
