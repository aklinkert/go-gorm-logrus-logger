# go-gorm-logrus-logger

[![test](https://github.com/aklinkert/go-gorm-logrus-logger/actions/workflows/test.yml/badge.svg)](https://github.com/aklinkert/go-gorm-logrus-logger/actions/workflows/test.yml)

A [GORM](https://gorm.io/) `logger.Interface` implementation that uses [logrus](https://github.com/sirupsen/logrus) for the actual logging.

- Queries are logged at debug level with the SQL as message and `rows` and `duration` (ms) fields.
- Failed queries and queries slower than the configured threshold are logged as warnings. `gorm.ErrRecordNotFound` is not logged.
- GORM's own messages are logged at info, warn and error level.
- `db.Logger.LogMode(...)` (and `db.Debug()`) is honoured, `logger.Silent` turns logging off.

## Installation

```sh
go get github.com/aklinkert/go-gorm-logrus-logger
```

## Usage

```go
package main

import (
	"time"

	gormlogruslogger "github.com/aklinkert/go-gorm-logrus-logger"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func main() {
	logger := logrus.New()

	// initialize your DB Connection
	var dial gorm.Dialector

	db, err := gorm.Open(dial, &gorm.Config{
		// Queries slower than 100ms are logged as warnings, 0 disables slow query logging.
		Logger: gormlogruslogger.NewGormLogrusLogger(logger.WithField("component", "gorm"), 100*time.Millisecond),
	})
	if err != nil {
		logger.Fatalf("failed to open DB connection: %v", err)
	}

	_ = db
}
```

To keep bound parameter values out of the logs, log the SQL with placeholders instead:

```go
gormlogruslogger.NewGormLogrusLogger(entry, 100*time.Millisecond, gormlogruslogger.WithParameterizedQueries())
```

## License

Apache 2.0, see [LICENSE](LICENSE).
