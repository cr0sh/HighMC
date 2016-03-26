package highmc

import "fmt"

// Try runs tryFunc, catches panic, and executes panicHandle with recovered panic.
func Try(tryFunc func(), panicHandle func(interface{})) {
	defer func() {
		if r := recover(); r != nil {
			panicHandle(r)
		}
	}()
	tryFunc()
	return
}

// Safe runs panicFunc, recovers panic if exists, and returns as error.
func Safe(panicFunc func()) error {
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", r)
			}
		}()
		panicFunc()
	}()
	return err
}
