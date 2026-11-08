package appexit

import (
	"errors"
	"testing"
)

func TestWithCode(t *testing.T) {
	err := WithCode(1, errors.New("fail"))
	var c CodedError
	if !errors.As(err, &c) || c.Code != 1 {
		t.Fatal(err)
	}
}
