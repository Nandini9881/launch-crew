package identity

import "time"

// SetExchangeTimeout shortens the default HTTP timeout for a test and restores it after.
func SetExchangeTimeout(d time.Duration) (restore func()) {
	old := exchangeTimeout
	exchangeTimeout = d
	return func() { exchangeTimeout = old }
}
