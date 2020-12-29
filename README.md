# go-gorm-logrus-logger

A simple logger implementation for [GORM](https://gorm.io/), using logrus for the actual logging.

## Usage

```go
package main

import (
	"context"

	"github.com/aklinkert/go-gorm-logrus-logger"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func main() {
	logger := logrus.New()

	// initialize your DB Connection
	var dial gorm.Dialector

	db, err := gorm.Open(dial, &gorm.Config{
		Logger: gormlogruslogger.NewGormLogrusLogger(logger.WithField("component", "gorm")),
	})

	if err != nil {
		logger.Fatalf("failed to open DB connection: %v", err)
	}

	var _ = db
}
```


## License

    Apache 2.0 Licence
