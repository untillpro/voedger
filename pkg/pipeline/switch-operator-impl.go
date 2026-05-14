// Copyright (c) 2021-present Voedger Authors.
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package pipeline

import "context"

type switchOperator struct {
	switchLogic ISwitch
	branches    map[string]ISyncOperator
}

func (s switchOperator) Close() {
	for _, branch := range s.branches {
		branch.Close()
	}
}

func (s switchOperator) DoSync(ctx context.Context, work IWorkpiece) (err error) {
	branchName, err := s.switchLogic.Switch(work)
	if err != nil {
		return err
	}
	return s.branches[branchName].DoSync(ctx, work)
}

type SwitchOperatorOptionFunc func(*switchOperator)

func SwitchOperator(switchLogic ISwitch, branch SwitchOperatorOptionFunc, branches ...SwitchOperatorOptionFunc) ISyncOperator {
	if switchLogic == nil {
		panic("switch must be not nil")
	}
	switchOperator := &switchOperator{
		switchLogic: switchLogic,
		branches:    make(map[string]ISyncOperator),
	}
	branch(switchOperator)
	for _, branch := range branches {
		branch(switchOperator)
	}
	return switchOperator
}

func SwitchBranch(name string, operator ISyncOperator) SwitchOperatorOptionFunc {
	return func(switchOperator *switchOperator) {
		switchOperator.branches[name] = operator
	}
}
