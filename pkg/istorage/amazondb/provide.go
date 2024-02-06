/*
 * Copyright (c) 2024-present unTill Pro, Ltd.
 * @author Alisher Nurmanov
 */

package amazondb

import "github.com/voedger/voedger/pkg/istorage"

func Provide(params DynamoDBParams) istorage.IAppStorageFactory {
	return &implIAppStorageFactory{
		params: params,
	}
}
