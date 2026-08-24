package main

import "testing"

func TestRequireLoopbackAddress(t *testing.T) {
	for _, addr := range []string{":50051", "0.0.0.0:50051", "[::]:50051", "localhost:50051"} {
		if err := requireLoopbackAddress(addr); err == nil {
			t.Fatalf("%q should be rejected", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1:50051", "[::1]:50051"} {
		if err := requireLoopbackAddress(addr); err != nil {
			t.Fatalf("%q should be accepted: %v", addr, err)
		}
	}
}
