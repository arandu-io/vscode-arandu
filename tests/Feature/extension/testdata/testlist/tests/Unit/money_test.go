package unit_test

import (
	"testing"
)

func TestMoney(t *testing.T) {
	t.Run("adds cents", func(t *testing.T) {
		t.Run("carries", func(t *testing.T) {})
	})
	t.Run("adds cents", func(t *testing.T) {})
	name := "dynamic"
	t.Run(name, func(t *testing.T) {})
}

/*
func TestCommentedOut(t *testing.T) {}
*/
var doc = `
func TestInRawString(t *testing.T) {}
`

func TestMain(m *testing.M) { m.Run() }

func Testify(t *testing.T) {}

type suite struct{}

func (s *suite) TestMethod(t *testing.T) {}

func TestBench(b *testing.B) {}

// func TestLineComment(t *testing.T) {}

func Test_underscore(t *testing.T) {}

func Test(t *testing.T) {}

func TestNoName(*testing.T) {}

func TestResult(t *testing.T) error { return nil }

func TestGeneric[T any](t *testing.T) {}

func TestLogin(t *testing.T) { t.Log("/* not a comment") }

func TestAfterString(t *testing.T) {}
