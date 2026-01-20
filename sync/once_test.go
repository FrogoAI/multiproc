package sync

import (
	"errors"
	"testing"
)

type one int

func (o *one) Increment() {
	*o++
}

func run(t *testing.T, once *Once, o *one, c chan bool) {
	err := once.Do(func() error {
		o.Increment()
		return nil
	})
	if err != nil {
		t.Errorf("once.Do() failed: %v", err)
	}

	if v := *o; v != 1 {
		t.Errorf("once failed inside run: %d is not 1", v)
	}
	c <- true
}

func TestOnce(t *testing.T) {
	o := new(one)
	once := new(Once)
	c := make(chan bool)
	const N = 10
	for i := 0; i < N; i++ {
		go run(t, once, o, c)
	}
	for i := 0; i < N; i++ {
		<-c
	}
	if *o != 1 {
		t.Errorf("once failed outside run: %d is not 1", *o)
	}
}

func TestOncePanic(t *testing.T) {
	var once Once
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("Once.Do did not panic")
			}
		}()
		once.Do(func() error {
			panic("failed")
			return nil
		})
	}()

	err := once.Do(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("once.Do() failed: %v", err)
	}
}

func TestOnceError(t *testing.T) {
	var once Once

	errOnce := errors.New("errOnce")

	err := once.Do(func() error {
		return errOnce
	})
	if err != errOnce {
		t.Errorf("once.Do() failed: %v", err)
	}
}

func BenchmarkOnce(b *testing.B) {
	var once Once
	f := func() error {
		return nil
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := once.Do(f)
			if err != nil {
				b.Errorf("once.Do() failed: %v", err)
			}
		}
	})
}
