-- Copyright (c) 2024-present unTill Pro, Ltd.
-- @author Alisher Nurmanov
IMPORT SCHEMA 'github.com/untillpro/airs-bp3/packages/air';

ALTERABLE WORKSPACE MyWorkspace3(
    TYPE MyType3(
        A varchar
    );
    TABLE MyTable3 INHERITS CDoc(
        Field1 varchar,
        Field2 int32
    );
);
