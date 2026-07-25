package concurrency

import "time"

func SafeSendBool(ch chan bool, value bool) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
		}
	}()

	ch <- value
	return false
}

func SafeSendString(ch chan string, value string) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
		}
	}()

	ch <- value
	return false
}

func SafeSendStringTimeout(ch chan string, value string, timeout int) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = false
		}
	}()

	select {
	case ch <- value:
		return true
	case <-time.After(time.Duration(timeout) * time.Second):
		return false
	}
}
