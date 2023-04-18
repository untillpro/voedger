/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package schemas

// Creates and return new application schemas cache
func NewSchemaCache() SchemaCacheBuilder {
	return newSchemaCache()
}
