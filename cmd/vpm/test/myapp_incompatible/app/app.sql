-- Copyright (c) 2023-present unTill Pro, Ltd.
-- @author Alisher Nurmanov

IMPORT SCHEMA 'mypkg1';
IMPORT SCHEMA 'mypkg2';
IMPORT SCHEMA 'github.com/voedger/voedger/pkg/registry';

APPLICATION test (
	USE mypkg1;
	USE mypkg2;
	USE registry;
);

TABLE MyTable INHERITS ODoc (
    myfield2 ref(mypkg2.MyTable2) NOT NULL,
    myfield3 ref(mypkg1.MyTable1) NOT NULL
);
