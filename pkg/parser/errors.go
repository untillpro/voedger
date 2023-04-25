/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */
package parser

import (
	"errors"
	"fmt"

	"github.com/alecthomas/participle/v2/lexer"
)

var ErrDirContainsNoSchemaFiles = errors.New("no schema files in directory")

func ErrUnexpectedSchema(fileName, actual, expected string) error {
	return fmt.Errorf("%s: package %s; expected %s", fileName, actual, expected)
}

var ErrFunctionParamsIncorrect = errors.New("function parameters do not match")
var ErrFunctionResultIncorrect = errors.New("function result do not match")
var ErrFunctionNotFound = errors.New("function not found")

func ErrSchemaContainsDuplicateName(name string) error {
	return fmt.Errorf("%s redeclared", name)
}

func errorAt(err error, pos *lexer.Position) error {
	return fmt.Errorf("%s: %w", pos.String(), err)
}
