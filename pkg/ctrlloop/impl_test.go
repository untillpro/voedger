/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Alisher Nurmanov
 */

package ctrlloop

import (
	"container/list"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/untillpro/goutils/logger"
)

func Test_BasicUsage(t *testing.T) {
	logger.SetLogLevel(logger.LogLevelVerbose)

	mockGetNextTimeFunc := func(cronSchedule string, startTimeTolerance time.Duration, nowTime time.Time) time.Time {
		return nowTime
	}

	nextStartTimeFunc = mockGetNextTimeFunc

	tests := []struct {
		name                string
		numReportedMessages int
		controller          ControllerFunction[string, int, string, int]
		messages            []OriginalMessage[string, int]
	}{
		{
			name:                "3 messages:A->B->C",
			numReportedMessages: 3,
			controller: func(key string, sp int, state string) (newState *string, pv *int, startTime *time.Time) {
				logger.Verbose("controllerFunc")
				v := 1
				pv = &v
				return nil, pv, nil
			},
			messages: []OriginalMessage[string, int]{
				{
					Key:                `A`,
					SP:                 0,
					CronSchedule:       `*/1 * * * *`,
					StartTimeTolerance: 5 * time.Second,
				},
				{
					Key:                `B`,
					SP:                 1,
					CronSchedule:       `now`,
					StartTimeTolerance: 5 * time.Second,
				},
				{
					Key:                `C`,
					SP:                 2,
					CronSchedule:       `*/1 * * * *`,
					StartTimeTolerance: 5 * time.Second,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wg := sync.WaitGroup{}

			mtx := sync.Mutex{}
			reportDB := make([]struct {
				Key string
				PV  *int
			}, 0)

			reporterFunc := func(key string, pv *int) (err error) {
				mtx.Lock()
				defer mtx.Unlock()

				logger.Verbose("reporterFunc")

				defer wg.Done()
				reportDB = append(reportDB, struct {
					Key string
					PV  *int
				}{Key: key, PV: pv})
				return nil
			}

			ch := make(chan OriginalMessage[string, int])

			New(test.controller, reporterFunc, 5, ch, time.Now)

			wg.Add(test.numReportedMessages)

			for _, m := range test.messages {
				ch <- OriginalMessage[string, int]{
					Key:                m.Key,
					SP:                 m.SP,
					CronSchedule:       m.CronSchedule,
					StartTimeTolerance: m.StartTimeTolerance,
				}
			}

			wg.Wait()

			close(ch)

			assert.GreaterOrEqual(t, test.numReportedMessages, len(reportDB))
		})
	}
}

func Test_SchedulerOnIn(t *testing.T) {
	mockGetNextTimeFunc := func(cronSchedule string, startTimeTolerance time.Duration, nowTime time.Time) time.Time {
		return nowTime
	}

	nextStartTimeFunc = mockGetNextTimeFunc

	tests := []struct {
		name             string
		originalMessages []OriginalMessage[string, int]
		scheduledItems   []scheduledMessage[string, int, struct{}]
		resultKeys       []string
		maxSerialNumber  uint64
	}{
		{
			name: `ok`,
			originalMessages: []OriginalMessage[string, int]{
				{
					Key:                `A`,
					SP:                 0,
					CronSchedule:       `*/1 * * * *`,
					StartTimeTolerance: 5 * time.Second,
				},
				{
					Key:                `B`,
					SP:                 1,
					CronSchedule:       `now`,
					StartTimeTolerance: 5 * time.Second,
				},
				{
					Key:                `C`,
					SP:                 2,
					CronSchedule:       `*/1 * * * *`,
					StartTimeTolerance: 5 * time.Second,
				},
				{
					Key:                `D`,
					SP:                 3,
					CronSchedule:       `*/1 * * * *`,
					StartTimeTolerance: 5 * time.Second,
				},
				{
					Key:                `E`,
					SP:                 4,
					CronSchedule:       `*/1 * * * *`,
					StartTimeTolerance: time.Second,
				},
			},
			scheduledItems: []scheduledMessage[string, int, struct{}]{
				{
					Key:          `A`,
					SP:           0,
					serialNumber: 1,
				},
				{
					Key:          `E`,
					SP:           1,
					serialNumber: 8,
				},
			},
			resultKeys:      []string{`A`, `B`, `C`, `D`, `E`},
			maxSerialNumber: 8,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initList := list.New()
			for _, i := range test.scheduledItems {
				initList.PushBack(i)
			}

			schedulerObj := newScheduler[string, int, struct{}](initList)

			for i, m := range test.originalMessages {
				schedulerObj.OnIn(uint64(i+1), m, time.Now())
			}

			maxSerialNumber := uint64(0)
			resultKeys := make([]string, 0)
			for element := schedulerObj.ScheduledItems.Front(); element != nil; element = element.Next() {
				m := element.Value.(scheduledMessage[string, int, struct{}])
				if m.serialNumber > maxSerialNumber {
					maxSerialNumber = m.serialNumber
				}
				resultKeys = append(resultKeys, m.Key)
			}

			sort.Strings(resultKeys)
			require.Equal(t, test.resultKeys, resultKeys)
			require.Equal(t, test.maxSerialNumber, maxSerialNumber)
		})
	}
}

func Test_SchedulerOnRepeat(t *testing.T) {
	mockGetNextTimeFunc := func(cronSchedule string, startTimeTolerance time.Duration, nowTime time.Time) time.Time {
		return nowTime
	}

	nextStartTimeFunc = mockGetNextTimeFunc

	tests := []struct {
		name             string
		messagesToRepeat []scheduledMessage[string, int, struct{}]
		resultKeys       []string
	}{
		{
			name: `ok`,
			messagesToRepeat: []scheduledMessage[string, int, struct{}]{
				{
					Key:          `A`,
					SP:           0,
					serialNumber: 1,
				},
				{
					Key:          `B`,
					SP:           1,
					serialNumber: 2,
				},
				{
					Key:          `C`,
					SP:           1,
					serialNumber: 2,
				},
			},
			resultKeys: []string{`A`, `B`, `C`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initList := list.New()
			schedulerObj := newScheduler[string, int, struct{}](initList)

			for _, m := range test.messagesToRepeat {
				schedulerObj.OnRepeat(m, time.Now())
			}

			resultKeys := make([]string, 0)
			for element := schedulerObj.ScheduledItems.Front(); element != nil; element = element.Next() {
				m := element.Value.(scheduledMessage[string, int, struct{}])
				resultKeys = append(resultKeys, m.Key)
			}

			sort.Strings(resultKeys)
			require.Equal(t, test.resultKeys, resultKeys)
		})
	}
}

func Test_SchedulerOnTimer(t *testing.T) {
	mockGetNextTimeFunc := func(cronSchedule string, startTimeTolerance time.Duration, nowTime time.Time) time.Time {
		return nowTime
	}

	nextStartTimeFunc = mockGetNextTimeFunc

	tests := []struct {
		name           string
		scheduledItems []scheduledMessage[string, int, struct{}]
	}{
		{
			name: `ok`,
			scheduledItems: []scheduledMessage[string, int, struct{}]{
				{
					Key:          `A`,
					SP:           0,
					serialNumber: 1,
				},
				{
					Key:          `E`,
					SP:           1,
					serialNumber: 8,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dedupInCh := make(chan statefulMessage[string, int, struct{}])

			initList := list.New()
			for _, i := range test.scheduledItems {
				initList.PushBack(i)
			}

			schedulerObj := newScheduler[string, int, struct{}](initList)

			go func() {
				for i := 0; i < len(test.scheduledItems); i++ {
					schedulerObj.OnTimer(dedupInCh, time.Now())
				}
			}()

			messageCounter := 0
			for range dedupInCh {
				messageCounter++
				if messageCounter == len(test.scheduledItems) {
					break
				}
			}

			// closing channels
			close(dedupInCh)
		})
	}
}

func Test_Dedupin(t *testing.T) {
	mockGetNextTimeFunc := func(cronSchedule string, startTimeTolerance time.Duration, nowTime time.Time) time.Time {
		return nowTime
	}

	nextStartTimeFunc = mockGetNextTimeFunc

	tests := []struct {
		name          string
		inProcessKeys []string
		messages      []statefulMessage[string, int, struct{}]
	}{
		{
			name:          `dedupIn<-A,B,B,C`,
			inProcessKeys: []string{``},
			messages: []statefulMessage[string, int, struct{}]{
				{
					Key:          `A`,
					SP:           0,
					serialNumber: 1,
				},
				{
					Key:          `B`,
					SP:           1,
					serialNumber: 1,
				},
				{
					Key:          `B`,
					SP:           2,
					serialNumber: 1,
				},
				{
					Key:          `C`,
					SP:           2,
					serialNumber: 1,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			InProcess := sync.Map{}
			dedupInCh := make(chan statefulMessage[string, int, struct{}])
			callerCh := make(chan statefulMessage[string, int, struct{}])
			repeatCh := make(chan scheduledMessage[string, int, struct{}], 3)

			go dedupIn(dedupInCh, callerCh, repeatCh, &InProcess, time.Now)

			go testMessagesWriter(dedupInCh, test.messages)

			callerMessageCounter := 0
			repeatMessageCounter := 0
			messageCounter := 0
			for messageCounter < len(test.messages) {
				messageCounter++
				select {
				case <-callerCh:
					callerMessageCounter++
				case <-repeatCh:
					repeatMessageCounter++
				}
			}

			// closing channels
			close(dedupInCh)
			close(repeatCh)

			if _, ok := <-callerCh; ok {
				t.Fatal(`callerCh channel must be closed`)
			}

			inProcessKeyCounter := 0
			InProcess.Range(func(_, _ any) bool {
				inProcessKeyCounter++
				return true
			})

			require.Equal(t, callerMessageCounter, inProcessKeyCounter)
			require.Equal(t, repeatMessageCounter, 1)
		})
	}
}

func Test_Repeater(t *testing.T) {
	mockGetNextTimeFunc := func(cronSchedule string, startTimeTolerance time.Duration, nowTime time.Time) time.Time {
		return nowTime
	}

	nextStartTimeFunc = mockGetNextTimeFunc

	now := time.Now()
	pv := 1

	tests := []struct {
		name          string
		inProcessKeys []string
		messages      []answer[string, int, int, struct{}]
	}{
		{
			name:          `repeater<-A,B,C,D`,
			inProcessKeys: []string{``},
			messages: []answer[string, int, int, struct{}]{
				{
					Key:          `A`,
					SP:           0,
					serialNumber: 1,
					StartTime:    &now,
					PV:           nil,
				},
				{
					Key:          `B`,
					SP:           1,
					serialNumber: 1,
					StartTime:    &now,
					PV:           nil,
				},
				{
					Key:          `C`,
					SP:           2,
					serialNumber: 1,
					StartTime:    nil,
					PV:           &pv,
				},
				{
					Key:          `D`,
					SP:           2,
					serialNumber: 1,
					StartTime:    nil,
					PV:           &pv,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repeaterCh := make(chan answer[string, int, int, struct{}])
			repeatCh := make(chan scheduledMessage[string, int, struct{}], 3)
			reporterCh := make(chan reportInfo[string, int])

			go repeater(repeaterCh, repeatCh, reporterCh)

			go testMessagesWriter(repeaterCh, test.messages)

			reporterMessageCounter := 0
			repeatMessageCounter := 0
			messageCounter := 0
			for messageCounter < len(test.messages) {
				messageCounter++
				select {
				case <-reporterCh:
					reporterMessageCounter++
				case <-repeatCh:
					repeatMessageCounter++
				}
			}

			// closing channels
			close(repeaterCh)

			if _, ok := <-repeatCh; ok {
				t.Fatal(`repeatCh channel must be closed`)
			}
			if _, ok := <-reporterCh; ok {
				t.Fatal(`reporterCh channel must be closed`)
			}

			require.Equal(t, reporterMessageCounter, 2)
			require.Equal(t, repeatMessageCounter, 2)
		})
	}
}

func testMessagesWriter[T any](ch chan<- T, arr []T) {
	for _, m := range arr {
		ch <- m
	}
}
