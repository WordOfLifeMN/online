package ytclient

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/googleapi"
)

// retryDelays are the waits between attempts at a write YouTube asked to be tried
// again, about half a minute in all. A playlist that was created moments ago refuses
// new items for a few seconds - "409 The operation was aborted" - before it settles.
// A variable so tests need not wait.
var retryDelays = []time.Duration{
	1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second,
}

// retryable reports whether an error is YouTube asking for the call to be tried again,
// rather than refusing it: an aborted operation, or the service being briefly
// unavailable. Anything else - a bad request, no permission, no quota - fails at once.
func retryable(err error) bool {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return false
	}

	switch apiErr.Code {
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}

	for _, item := range apiErr.Errors {
		switch item.Reason {
		case "backendError", "SERVICE_UNAVAILABLE":
			return true
		}
	}
	return apiErr.Code == http.StatusConflict && strings.Contains(strings.ToLower(apiErr.Message), "aborted")
}

// isNotFound reports YouTube saying a resource does not exist
func isNotFound(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound
}

// retryableFor is retryable, widened for a video this run uploaded: YouTube can say
// such a video does not exist for a little while after it does
func (c *Client) retryableFor(videoID string) func(error) bool {
	return func(err error) bool {
		return retryable(err) || (c.wasUploaded(videoID) && isNotFound(err))
	}
}

// withRetry runs a write, trying it again with growing waits while YouTube asks for
// that - while canRetry says so.
//
// done is asked before each retry whether the write took effect after all, so that an
// attempt that was reported as failed but was in fact applied is not repeated. A write
// that is not idempotent - adding to a playlist - must supply it; done may be nil for
// one that is.
func (c *Client) withRetry(
	what string,
	canRetry func(error) bool,
	write func() error,
	done func() (bool, error),
) error {
	err := write()
	for _, delay := range retryDelays {
		if err == nil || !canRetry(err) {
			return err
		}

		fmt.Fprintf(c.Progress, "  YouTube is not ready to %s yet, retrying in %s...\n", what, delay)
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		case <-time.After(delay):
		}

		if done != nil {
			applied, checkErr := done()
			if checkErr == nil && applied {
				return nil
			}
		}
		err = write()
	}
	if err != nil && canRetry(err) {
		return fmt.Errorf("YouTube was still not ready after %d attempts: %w", len(retryDelays)+1, err)
	}
	return err
}
