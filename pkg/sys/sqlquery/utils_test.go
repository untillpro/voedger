/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package sqlquery

import (
	"reflect"
	"testing"

	"github.com/voedger/voedger/pkg/istructs"
)

func Test_parseQueryAppWs(t *testing.T) {
	type args struct {
		query string
	}
	tests := []struct {
		name      string
		args      args
		wantApp   istructs.AppQName
		wantWs    istructs.WSID
		wantClean string
		wantErr   bool
	}{
		{"fail: empty", args{""}, istructs.NullAppQName, 0, "", true},
		{"fail: no table", args{"select * from"}, istructs.NullAppQName, 0, "", true},
		{"fail: missed select", args{"query * from table"}, istructs.NullAppQName, 0, "", true},
		{"fail: missed from", args{"select * table"}, istructs.NullAppQName, 0, "", true},

		{"OK:", args{"select * from table"}, istructs.NullAppQName, 0, "select * from table", false},
		{"OK:", args{"select * from owner.app.table"}, istructs.MustParseAppQName("owner/app"), 0, "select * from table", false},
		{"OK:", args{"select * from owner.app.123.table"}, istructs.MustParseAppQName("owner/app"), 123, "select * from table", false},
		{"OK:", args{"select * from 123.table"}, istructs.NullAppQName, 123, "select * from table", false},

		{"OK:", args{"select f1, f2 from table where f3 is null"}, istructs.NullAppQName, 0, "select f1, f2 from table where f3 is null", false},
		{"OK:", args{"select f1, f2 from owner.app.123.table where f3 is null"}, istructs.MustParseAppQName("owner/app"), 123, "select f1, f2 from table where f3 is null", false},

		{"fail: invalid app name", args{"select naked.🔫.table"}, istructs.NullAppQName, 0, "", true},
		{"fail: invalid table name", args{"select ups.table"}, istructs.NullAppQName, 0, "", true},
		{"fail: invalid ws", args{"select -123.table"}, istructs.NullAppQName, 0, "", true},
		{"fail: invalid app or ws", args{"select owner.app.ooo.table"}, istructs.NullAppQName, 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotApp, gotWs, gotClean, err := parseQueryAppWs(tt.args.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseQueryAppWs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotApp, tt.wantApp) {
				t.Errorf("parseQueryAppWs() gotApp = %v, want %v", gotApp, tt.wantApp)
			}
			if !reflect.DeepEqual(gotWs, tt.wantWs) {
				t.Errorf("parseQueryAppWs() gotWs = %v, want %v", gotWs, tt.wantWs)
			}
			if gotClean != tt.wantClean {
				t.Errorf("parseQueryAppWs() gotClean = %v, want %v", gotClean, tt.wantClean)
			}
		})
	}
}
