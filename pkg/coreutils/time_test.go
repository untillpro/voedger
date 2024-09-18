/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package coreutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTime_BasicUsage(t *testing.T) {
	require := require.New(t)
	tm := NewITime()

	t.Run("now", func(t *testing.T) {
		now := tm.Now()
		require.Less(now.UnixMilli()-time.Now().UnixMilli(), int64(10*time.Millisecond))
	})

	t.Run("timer", func(t *testing.T) {
		timer := tm.NewTimerChan(100 * time.Millisecond)
		start := tm.Now()
		firingInstant := <-timer
		require.Less(firingInstant.UnixMilli()-start.UnixMilli(), int64(100+30)) // 30 msecs lag for slow PCs
	})
}

func TestMockTime(t *testing.T) {
	require := require.New(t)
	t.Run("now", func(t *testing.T) {
		tm1 := MockTime.Now()
		time.Sleep(10 * time.Millisecond)
		tm2 := MockTime.Now()
		require.Equal(tm1, tm2)
	})

	t.Run("add", func(t *testing.T) {
		tm1 := MockTime.Now()
		MockTime.Add(1 * time.Minute)
		tm2 := MockTime.Now()
		require.Equal(time.Minute, tm2.Sub(tm1))
	})

	t.Run("timer", func(t *testing.T) {
		timer1 := MockTime.NewTimerChan(123 * time.Hour)
		timer2 := MockTime.NewTimerChan(125 * time.Hour)
		MockTime.Add(122 * time.Hour)
		select {
		case <-timer1:
			t.Fatal(1)
		case <-timer2:
			t.Fatal(2)
		default:
		}

		MockTime.Add(1 * time.Hour) // +123 hours

		var firingInstant time.Time
		select {
		case firingInstant = <-timer1:
		case <-timer2:
			t.Fatal(3)
		default:
			t.Fatal(4)
		}

		require.Equal(MockTime.Now(), firingInstant)

		// cross over timer2
		MockTime.Add(3 * time.Hour) // +126 hours

		select {
		case <-timer1:
			t.Fatal(5)
		case firingInstant = <-timer2:
		default:
			t.Fatal(6)
		}

		require.Equal(MockTime.Now(), firingInstant)
	})
}

func TestMockTimer(t *testing.T) {
	timer := MockTime.NewTimerChan(time.Hour)
	MockTime.Add(59 * time.Minute)
	select {
	case <-timer:
		t.Fatal(1)
	default:
	}
	MockTime.Add(time.Minute)
	firingInstant := <-timer
	require.Equal(t, firingInstant, MockTime.Now())
}

func BenchmarkMockTimers(b *testing.B) {
	b.Run("create only", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = MockTime.NewTimerChan(time.Hour)
		}
	})

	b.Run("create and fire", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = MockTime.NewTimerChan(time.Hour)
			MockTime.Add(time.Hour)
		}
	})

	b.Run("create, fire and read", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			timerChan := MockTime.NewTimerChan(time.Hour)
			MockTime.Add(time.Hour)
			<-timerChan
		}
	})
}

func BenchmarkRealTimers(b *testing.B) {
	b.Run("create only", func(b *testing.B) {
		t := NewITime()
		for i := 0; i < b.N; i++ {
			_ = t.NewTimerChan(time.Hour)
		}
	})

	b.Run("create and fire", func(b *testing.B) {
		t := NewITime()
		for i := 0; i < b.N; i++ {
			_ = t.NewTimerChan(time.Millisecond)
			time.Sleep(2 * time.Millisecond)
		}
	})

	b.Run("create, fire and read", func(b *testing.B) {
		t := NewITime()
		for i := 0; i < b.N; i++ {
			timerChan := t.NewTimerChan(time.Millisecond)
			time.Sleep(2 * time.Millisecond)
			<-timerChan
		}
	})
}
