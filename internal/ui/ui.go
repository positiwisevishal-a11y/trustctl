package ui

import (
	"fmt"
	"os"
)

func Info(format string, a ...interface{}) {
	fmt.Printf("ℹ️  "+format+"\n", a...)
}

func Success(format string, a ...interface{}) {
	fmt.Printf("✅ "+format+"\n", a...)
}

func Warning(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "⚠️  "+format+"\n", a...)
}

func Error(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "❌ "+format+"\n", a...)
}

func StepStart(format string, a ...interface{}) {
	fmt.Printf("🔄 "+format+"\n", a...)
}

func StepDone(format string, a ...interface{}) {
	fmt.Printf("✔️  "+format+"\n", a...)
}
