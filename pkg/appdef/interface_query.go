/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// Query is a function that returns data from system state.
type IQuery interface {
	IFunction

	// Unwanted type assertion stub
	isQuery()
}

type IQueryBuilder interface {
	IFunctionBuilder
}
