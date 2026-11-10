package main

import "log"

func logInfo(scope, format string, v ...any) {
	log.Printf("[%s] "+format, append([]any{scope}, v...)...)
}

func logWarn(scope, format string, v ...any) {
	log.Printf("[%s] WARNING: "+format, append([]any{scope}, v...)...)
}

func logError(scope, format string, v ...any) {
	log.Printf("[%s] ERROR: "+format, append([]any{scope}, v...)...)
}

func logStep(scope, format string, v ...any) {
	log.Printf("[%s] "+format, append([]any{scope}, v...)...)
}
