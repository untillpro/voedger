-- Copyright (c) 2020-present unTill Pro, Ltd.

-- note: this schema is for tests only. Voedger sys package uses copy of this schema

ABSTRACT TABLE CRecord();
ABSTRACT TABLE WRecord();
ABSTRACT TABLE ORecord();

ABSTRACT TABLE CDoc INHERITS CRecord();
ABSTRACT TABLE ODoc INHERITS ORecord();
ABSTRACT TABLE WDoc INHERITS WRecord();

ABSTRACT TABLE Singleton INHERITS CDoc();

ABSTRACT WORKSPACE Workspace (
    TYPE CreateLoginParams(
        Login                       varchar,
        AppName                     varchar,
        SubjectKind                 int32,
        WSKindInitializationData    varchar(1024),
        ProfileCluster              int32
    );
    TYPE CreateLoginUnloggedParams(
        Password varchar
    );
    EXTENSION ENGINE BUILTIN (
        COMMAND CreateLogin(CreateLoginParams, UNLOGGED CreateLoginUnloggedParams) RETURNS void;
        COMMAND UpdateSubscription(any) RETURNS any;
        QUERY UPTerminalWebhook(any) RETURNS any;
    )
);

ALTERABLE WORKSPACE Profile(

);

/*
    Specify sys.Raw as a command/function parameter to pass raw data to the command/function. 
    The data isn't valudated by the core.
    An extension accesses the data by reading the "Body" field from the argument.
*/
TYPE Raw(
    Body   varchar(65535)
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
