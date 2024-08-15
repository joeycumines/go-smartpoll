package smartpoll

import "testing"

func Test_startTimer_zero(t *testing.T) {
	if v := startTimer(0); v != readySentinel {
		t.Error(v)
	}
}

func Test_stopTimer_readySentinel(t *testing.T) {
	if !stopTimer(&readySentinel) {
		t.Error()
	}
}
