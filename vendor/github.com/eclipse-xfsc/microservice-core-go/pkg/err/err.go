package err

import (
	"fmt"
	"io"

	"github.com/eclipse-xfsc/microservice-core-go/pkg/logr"
)

func LogChan(logger logr.Logger, ch <-chan error) {
	for {
		err, open := <-ch

		if !open {
			return
		}

		logger.Error(err, "error received from err chan")
	}
}

// LogChanToWriter can be used to send any error coming through
// the given chan error to the io.Writer.
//
// Usage with ConnectRetry:
//
// errChan := make(chan error)
// go LogErrChan(logger, errChan)
// if err := ConnectRetry; err != nil { ... }
func LogChanToWriter(w io.Writer, ch <-chan error) {
	for {
		err, open := <-ch

		if !open {
			return
		}

		fmt.Fprintf(w, "%+v", err)
	}
}
