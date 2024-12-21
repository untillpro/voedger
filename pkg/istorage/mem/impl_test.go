/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package mem

import (
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/istorage"
	"testing"
)

func TestMemTCK(t *testing.T) {
	mockSleeper := coreutils.NewMockTimeSleeper()
	istorage.TechnologyCompatibilityKit(t, Provide(coreutils.MockTime, mockSleeper), mockSleeper)
}
