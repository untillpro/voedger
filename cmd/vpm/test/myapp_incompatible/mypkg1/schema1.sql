-- Copyright (c) 2023-present unTill Pro, Ltd.
-- @author Alisher Nurmanov

TABLE MyTable1 INHERITS ODoc (
    myfield1 int64 NOT NULL -- allowed: type changed from int32 to int64
);

TYPE mytype (
    field text NOT NULL,
    newField text NOT NULL -- allowed: new field
);
