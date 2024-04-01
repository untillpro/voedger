-- Copyright (c) 2023-present unTill Pro, Ltd.
-- @author Alisher Nurmanov

IMPORT SCHEMA 'mypkg1';
IMPORT SCHEMA 'mypkg2';

APPLICATION APP(
    USE mypkg1;
    USE mypkg2;
);

WORKSPACE MyAppWorkspace INHERITS mypkg1.MyWorkspace1(
    TABLE MyAppTable INHERITS CDoc(
        FieldA varchar,
        FieldB int32
    );
	EXTENSION ENGINE BUILTIN (
        COMMAND CmdApp(sys.Raw) RETURNS sys.Raw;
	);
);
