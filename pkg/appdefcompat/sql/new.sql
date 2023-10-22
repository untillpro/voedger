-- noinspection SqlNoDataSourceInspectionForFile

-- Copyright (c) 2020-present unTill Pro, Ltd.

-- note: this schema is for tests only. Voedger sys package uses copy of this schema
APPLICATION TEST();

ABSTRACT TABLE CRecord();
ABSTRACT TABLE WRecord();
ABSTRACT TABLE ORecord();

ABSTRACT TABLE CDoc INHERITS CRecord();
ABSTRACT TABLE ODoc INHERITS ORecord();
ABSTRACT TABLE WDoc INHERITS WRecord();

ABSTRACT TABLE Singleton INHERITS CDoc();

WORKSPACE Workspace ( -- ValueChanged: Abstract flag must not be changed
);

WORKSPACE SomeWorkspace(
    TYPE CreateLoginUnloggedParams(
        Password varchar, -- OrderChanged
        Email varchar --OrderChanged
    );
    TYPE CreateLoginParams(
        --Login                     varchar, -- NodeRemoved
        AppName                     varchar,
        SubjectKind                 int32,
        WSKindInitializationData    varchar(1024),
        ProfileCluster              int64, -- ValueChanged: int32 in old version, int64 in new version
        ProfileToken                int64, -- ValueChanged: int32 in old version, int64 in new version
        NewField                    varchar -- appending field is allowed
    );
    TABLE AnotherOneTable INHERITS CDoc(
        A varchar,
        B varchar,
        D varchar, -- NodeInserted
        C int32 -- OrderChanged, ValueChanged: varchar in old version, int32 in new version, field's index is changed
    );
    TYPE SomeType(
        A varchar,
        B int
    );
    TYPE SomeType2(
        A varchar,
        B int,
        C int,
        D int
    );
    VIEW SomeView(
        A int,
        B varchar, -- ValueChanged: field B was changed as a part of ClustColsFields
        C int, -- actual error raises in primary key
        D int, -- order of this field is not changed because it is value field
        E varchar, -- ValueChanged: field E was changed as a part of value fields
        F int, -- appending field is allowed to value fields
        PRIMARY KEY ((A, C), B) -- NodeModified: added field C to PartKeyFields
    ) AS RESULT OF Proj1;
    TABLE NewTable INHERITS CDoc(
        A varchar
    );
    TYPE NewType( -- new type is allowed
        A varchar
    );
    TYPE NewType2( -- new type is allowed
        A varchar,
        B int32
    );
    VIEW NewView( -- new view is allowed
        A int,
        B int,
        PRIMARY KEY ((A), B)
    ) AS RESULT OF Proj1;
    EXTENSION ENGINE BUILTIN (
        PROJECTOR Proj1 ON (Orders) INTENTS (View(SomeView), View(NewView));
        COMMAND Orders();
        COMMAND CreateLogin(CreateLoginParams, UNLOGGED CreateLoginUnloggedParams) RETURNS void;
        COMMAND SomeCommand(SomeType2, UNLOGGED SomeType2) RETURNS SomeType2; -- args and return type changed; unlogged flag changed, but it is ok
        COMMAND NewCommand(NewType, UNLOGGED NewType2) RETURNS NewType;
        QUERY NewQuery(NewType) RETURNS NewType; -- new query is allowed
        QUERY SomeQuery(SomeType2) RETURNS SomeType2; -- changing args and return type is allowed
    )
);

ALTERABLE WORKSPACE Profile(

);

EXTENSION ENGINE BUILTIN (

    STORAGE Record(
        GET         SCOPE(COMMANDS, QUERIES, PROJECTORS),
        GETBATCH    SCOPE(COMMANDS, QUERIES, PROJECTORS),
        INSERT      SCOPE(COMMANDS),
        UPDATE      SCOPE(COMMANDS)
    ) ENTITY RECORD; -- used to validate projector state/intents declaration


    STORAGE View(
        GET         SCOPE(COMMANDS, QUERIES, PROJECTORS),
        GETBATCH    SCOPE(COMMANDS, QUERIES, PROJECTORS),
        READ        SCOPE(QUERIES, PROJECTORS),
        INSERT      SCOPE(PROJECTORS),
        UPDATE      SCOPE(PROJECTORS)
    ) ENTITY VIEW;

    STORAGE WLog(
        GET     SCOPE(COMMANDS, QUERIES, PROJECTORS),
        READ    SCOPE(QUERIES, PROJECTORS)
    );

    STORAGE PLog(
        GET     SCOPE(COMMANDS, QUERIES, PROJECTORS),
        READ    SCOPE(QUERIES, PROJECTORS)
    );

    STORAGE AppSecret(
        GET SCOPE(COMMANDS, QUERIES, PROJECTORS)
    );

    STORAGE Subject(
        GET SCOPE(COMMANDS, QUERIES)
    );

    STORAGE Http (
        READ SCOPE(QUERIES, PROJECTORS)
    );

    STORAGE SendMail(
        INSERT SCOPE(PROJECTORS)
    );

    STORAGE CmdResult(
        INSERT SCOPE(COMMANDS)
    );

)
