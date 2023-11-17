/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package istructsmem

import (
	"context"
	"errors"
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
)

// Implements istructs.IResources
type Resources struct {
	cfg       *AppConfigType
	resources map[appdef.QName]istructs.IResource
}

func newResources(cfg *AppConfigType) Resources {
	return Resources{cfg, make(map[appdef.QName]istructs.IResource)}
}

// Adds new resource to application resources
func (res *Resources) Add(r istructs.IResource) *Resources {
	res.resources[r.QName()] = r
	return res
}

// Finds application resource by QName
func (res *Resources) QueryResource(resource appdef.QName) (r istructs.IResource) {
	r, ok := res.resources[resource]
	if !ok {
		return nullResource
	}
	return r
}

// Returns argument object builder for query function
func (res *Resources) QueryFunctionArgsBuilder(query istructs.IQueryFunction) istructs.IObjectBuilder {
	return newObject(res.cfg, query.ParamsType(), nil)
}

// Returns command function from application resource by QName or nil if not founded
func (res *Resources) CommandFunction(name appdef.QName) (cmd istructs.ICommandFunction) {
	r := res.QueryResource(name)
	if r.Kind() == istructs.ResourceKind_CommandFunction {
		cmd := r.(istructs.ICommandFunction)
		return cmd
	}
	return nil
}

// Enumerates all application resources
func (res *Resources) Resources(enum func(appdef.QName)) {
	for n := range res.resources {
		enum(n)
	}
}

// Checks that all resources use correct QNames
func (res *Resources) prepare(app appdef.IAppDef) (err error) {

	// https://dev.heeus.io/launchpad/#!17185
	checkParam := func(r, n appdef.QName, par string) {
		if (n != appdef.NullQName) && (n != appdef.QNameANY) {
			t := app.TypeByName(n)
			if t == nil {
				err = errors.Join(err,
					fmt.Errorf("resource «%v» uses unknown type «%v» as %s", r, n, par))
			} else {
				switch t.Kind() {
				case appdef.TypeKind_Data, appdef.TypeKind_ODoc, appdef.TypeKind_Object: // ok
				default:
					err = errors.Join(err, fmt.Errorf("resource «%v» uses non-object type %v as %s", r, t, par))
				}
			}
		}
	}

	checkResult := func(r, n appdef.QName) {
		if (n != appdef.NullQName) && (n != appdef.QNameANY) {
			t := app.TypeByName(n)
			if t == nil {
				err = errors.Join(err,
					fmt.Errorf("resource «%v» uses unknown type «%v» as result", r, n))
			} else {
				switch t.Kind() {
				case appdef.TypeKind_Data, appdef.TypeKind_GDoc, appdef.TypeKind_CDoc, appdef.TypeKind_WDoc, appdef.TypeKind_ODoc, appdef.TypeKind_Object: // ok
				default:
					err = errors.Join(err, fmt.Errorf("resource «%v» uses non-object type %v as result", r, t))
				}
			}
		}
	}

	res.Resources(func(n appdef.QName) {
		r := res.QueryResource(n)
		switch r.Kind() {
		case istructs.ResourceKind_QueryFunction:
			q := r.(istructs.IQueryFunction)
			checkParam(q.QName(), q.ParamsType(), "parameter")
			checkResult(q.QName(), q.ResultType(nullPrepareArgs))
		case istructs.ResourceKind_CommandFunction:
			c := r.(istructs.ICommandFunction)
			checkParam(c.QName(), c.ParamsType(), "parameter")
			checkParam(c.QName(), c.UnloggedParamsType(), "unlogged parameter")
			checkResult(c.QName(), c.ResultType())
		}
	})
	return err
}

// Ancestor for command & query functions
type abstractFunction struct {
	name, pars appdef.QName
	res        func(istructs.PrepareArgs) appdef.QName
}

// istructs.IResource
func (af *abstractFunction) QName() appdef.QName { return af.name }

// istructs.IFunction
func (af *abstractFunction) ParamsType() appdef.QName { return af.pars }

// istructs.IFunction
func (af *abstractFunction) ResultType(args istructs.PrepareArgs) appdef.QName {
	return af.res(args)
}

// For debug and logging purposes
func (af *abstractFunction) String() string {
	return fmt.Sprintf("%v", af.QName())
}

type (
	// Function type to call for query execute action
	ExecQueryClosure func(ctx context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error)

	// Implements istructs.IQueryFunction
	queryFunction struct {
		abstractFunction
		exec ExecQueryClosure
	}
)

// Creates and returns new query function
func NewQueryFunction(name, pars, result appdef.QName, exec ExecQueryClosure) istructs.IQueryFunction {
	return NewQueryFunctionCustomResult(name, pars, func(istructs.PrepareArgs) appdef.QName { return result }, exec)
}

func NewQueryFunctionCustomResult(name, pars appdef.QName, resultFunc func(istructs.PrepareArgs) appdef.QName, exec ExecQueryClosure) istructs.IQueryFunction {
	return &queryFunction{
		abstractFunction: abstractFunction{
			name: name,
			pars: pars,
			res:  resultFunc,
		},
		exec: exec,
	}
}

// Null execute action closure for query functions
func NullQueryExec(_ context.Context, _ istructs.ExecQueryArgs, _ istructs.ExecQueryCallback) error {
	return nil
}

// istructs.IQueryFunction
func (qf *queryFunction) Exec(ctx context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
	return qf.exec(ctx, args, callback)
}

// istructs.IResource
func (qf *queryFunction) Kind() istructs.ResourceKindType {
	return istructs.ResourceKind_QueryFunction
}

// istructs.IQueryFunction
func (qf *queryFunction) ResultType(args istructs.PrepareArgs) appdef.QName {
	return qf.abstractFunction.ResultType(args)
}

// for debug and logging purposes
func (qf *queryFunction) String() string {
	return fmt.Sprintf("q:%v", qf.abstractFunction.String())
}

type (
	// Function type to call for command execute action
	ExecCommandClosure func(args istructs.ExecCommandArgs) (err error)

	// Implements istructs.ICommandFunction
	commandFunction struct {
		abstractFunction
		unlPars appdef.QName
		exec    ExecCommandClosure
	}
)

// NewCommandFunction creates and returns new command function
func NewCommandFunction(name, params, unlogged, result appdef.QName, exec ExecCommandClosure) istructs.ICommandFunction {
	return &commandFunction{
		abstractFunction: abstractFunction{
			name: name,
			pars: params,
			res:  func(pa istructs.PrepareArgs) appdef.QName { return result },
		},
		unlPars: unlogged,
		exec:    exec,
	}
}

// NullCommandExec is null execute action closure for command functions
func NullCommandExec(_ istructs.ExecCommandArgs) error {
	return nil
}

// istructs.ICommandFunction
func (cf *commandFunction) Exec(args istructs.ExecCommandArgs) error {
	return cf.exec(args)
}

// istructs.IResource
func (cf *commandFunction) Kind() istructs.ResourceKindType {
	return istructs.ResourceKind_CommandFunction
}

// istructs.ICommandFunction
func (cf *commandFunction) ResultType() appdef.QName {
	return cf.abstractFunction.ResultType(nullPrepareArgs)
}

// for debug and logging purposes
func (cf *commandFunction) String() string {
	return fmt.Sprintf("c:%v", cf.abstractFunction.String())
}

func (cf *commandFunction) UnloggedParamsType() appdef.QName {
	return cf.unlPars
}

// nullResourceType type to return then resource is not founded
//   - interfaces:
//     — IResource
type nullResourceType struct {
}

func newNullResource() *nullResourceType {
	return &nullResourceType{}
}

// IResource members
func (r *nullResourceType) Kind() istructs.ResourceKindType { return istructs.ResourceKind_null }
func (r *nullResourceType) QName() appdef.QName             { return appdef.NullQName }
