/*
 * Copyright (c) 2023-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

import (
	"errors"
	"fmt"
)

// # Implements:
//   - ICommand
type command struct {
	function
	unl typeRef
}

func newCommand(app *appDef, name QName) *command {
	cmd := &command{}
	cmd.function = makeFunc(app, name, TypeKind_Command)
	app.appendType(cmd)
	return cmd
}

func (cmd *command) UnloggedParam() IType {
	return cmd.unl.target(cmd.app)
}

// Validates command
func (cmd *command) Validate() (err error) {
	err = cmd.function.Validate()

	if ok, e := cmd.unl.valid(cmd.app); !ok {
		err = errors.Join(err, fmt.Errorf("%v: invalid or unknown unlogged parameter type: %w", cmd, e))
	} else if typ := cmd.UnloggedParam(); typ != nil {
		switch typ.Kind() {
		case TypeKind_Any: // ok
		case TypeKind_Data, TypeKind_ODoc, TypeKind_Object: // ok
		default:
			err = errors.Join(err, fmt.Errorf("%v: unlogged parameter type is %v, must be ODoc, Object or Data: %w", cmd, typ, ErrInvalidTypeKind))
		}
	}

	return err
}

func (cmd *command) setUnloggedParam(name QName) {
	cmd.unl.setName(name)
}

type commandBuilder struct {
	functionBuilder
	*command
}

func newCommandBuilder(command *command) *commandBuilder {
	return &commandBuilder{
		functionBuilder: makeFunctionBuilder(&command.function),
		command:         command,
	}
}

func (cb *commandBuilder) SetUnloggedParam(name QName) ICommandBuilder {
	cb.command.setUnloggedParam(name)
	return cb
}
