-- Copyright (c) 2024-present unTill Pro, Ltd.
-- @author Alisher Nurmanov

IMPORT SCHEMA 'mypkg1';

WORKSPACE MyWorkspace2 INHERITS mypkg1.MyWorkspace1(
    TABLE MyTable2 INHERITS CDoc(
        Field3 varchar,
        Field4 int32
    );
);
