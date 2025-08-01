package errorhandlefunc

import "gotulua/pagesfunc"

func ShowDataError(msg string, doPanic bool) {
	if doPanic {
		panic(msg)
	}
	pagesfunc.ErrorMessage(msg)
}
