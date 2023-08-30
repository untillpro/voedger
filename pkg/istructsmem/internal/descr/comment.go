/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package descr

import "github.com/voedger/voedger/pkg/appdef"

func readComment(c interface{}) (text string) {
	if comment, ok := c.(appdef.IComment); ok {
		text = comment.Comment()
	}
	return text
}
