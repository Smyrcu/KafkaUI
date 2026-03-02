package auth

import (
	"testing"
	"time"
)

func TestLoginRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewLoginRateLimiter(5, time.Minute)
	for i := 0; i < 5; i++ {
		if !rl.Allow("192.168.1.1") {
			t.Fatalf("expected attempt %d to be allowed", i+1)
		}
	}
}

func TestLoginRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewLoginRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		rl.Allow("192.168.1.1")
	}
	if rl.Allow("192.168.1.1") {
		t.Fatal("expected 4th attempt to be blocked")
	}
}

func TestLoginRateLimiter_SeparateKeys(t *testing.T) {
	rl := NewLoginRateLimiter(2, time.Minute)
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.1")

	if !rl.Allow("192.168.1.2") {
		t.Fatal("expected different IP to be allowed")
	}
}

func TestLoginRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := NewLoginRateLimiter(2, 50*time.Millisecond)
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.1")

	if rl.Allow("192.168.1.1") {
		t.Fatal("expected to be blocked")
	}

	time.Sleep(60 * time.Millisecond)

	if !rl.Allow("192.168.1.1") {
		t.Fatal("expected to be allowed after window reset")
	}
}
