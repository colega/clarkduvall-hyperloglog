package hll

import (
	"testing"
	"bytes"
)

func TestVariableLengthList(t *testing.T) {
	l := make(variableLengthList, 0, 100)

	l = l.Append(106903)

	l2 := []uint8{134, 195, 23}
	if bytes.Compare(l, l2) != 0 { t.Error(l) }

	l = l.Append(0x7f)
	l2 = append(l2, 0x7f)
	if bytes.Compare(l, l2) != 0 { t.Error(l) }

	l = l.Append(0xff)
	l2 = append(l2, 0x81, 0x7f)
	if bytes.Compare(l, l2) != 0 { t.Error(l) }

	l = l.Append(0xffffffff)
	l2 = append(l2, 0x8f, 0xff, 0xff, 0xff, 0x7f)
	if bytes.Compare(l, l2) != 0 { t.Error(l) }

	iter := l.Iter()

	n := iter.Peek()
	if n != 106903 { t.Error(n) }

	n = iter.Next()
	if n != 106903 { t.Error(n) }

	n = iter.Next()
	if n != 0x7f { t.Error(n) }

	n = iter.Next()
	if n != 0xff { t.Error(n) }

	n = iter.Next()
	if n != 0xffffffff { t.Error(n) }
}

func TestCompressedList(t *testing.T) {
	l := newCompressedList(100)

	l.Append(0xff)

	iter := l.Iter()

	n := iter.Peek()
	if n != 0xff { t.Error(n) }

	n = iter.Next()
	if n != 0xff { t.Error(n) }

	l.Append(0xffffffff)
	n = iter.Peek()
	if n != 0xffffffff { t.Error(n) }
	n = iter.Next()
	if n != 0xffffffff { t.Error(n) }

	l.Append(0xffff)
	n = iter.Next()
	if n != 0xffff { t.Error(n) }

	l.Append(0xb0af1000)
	n = iter.Next()
	if n != 0xb0af1000 { t.Error(n) }

	iter = l.Iter()
	n = iter.Next()
	if n != 0xff { t.Error(n) }
	n = iter.Next()
	if n != 0xffffffff { t.Error(n) }
	n = iter.Next()
	if n != 0xffff { t.Error(n) }
	n = iter.Next()
	if n != 0xb0af1000 { t.Error(n) }
}
