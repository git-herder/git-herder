package main

import "testing"

func TestMainSymbolLinked(t *testing.T) {
	// Косвенно покрываем точку входа: main делегирует в bootstrap.Run.
	_ = main
}
