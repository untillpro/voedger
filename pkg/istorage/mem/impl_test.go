/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package mem

import (
	"testing"

	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/istorage"
)

func TestMemTCK(t *testing.T) {
	istorage.TechnologyCompatibilityKit(t, Provide(coreutils.MockTime))
}
