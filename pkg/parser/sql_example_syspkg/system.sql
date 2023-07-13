-- note: this schema is for tests only. Voedger sys package uses copy of this schema
SCHEMA test_sys;
TABLE CDoc();
TABLE ODoc();
TABLE WDoc();
TABLE Singleton INHERITS CDoc();
TABLE CRecord();
TABLE WRecord();
TABLE ORecord();

EXTENSION ENGINE BUILTIN (

    STORAGE Record( 
        GET BATCH,
        INSERT SCOPE COMMANDS,
        UPDATE SCOPE COMMANDS
    ) REQUIRES ENTITY;
    

    STORAGE View(
        GET BATCH,
        READ SCOPE QUERIES AND PROJECTORS,
        INSERT SCOPE PROJECTORS,
        UPDATE SCOPE PROJECTORS
    ) REQUIRES ENTITY;

    STORAGE WLog(
        GET,
        READ SCOPE QUERIES AND PROJECTORS
    );

    STORAGE PLog(
        GET,
        READ SCOPE QUERIES AND PROJECTORS
    );

    STORAGE AppSecrets(GET);

    STORAGE Subject(GET SCOPE COMMANDS AND QUERIES);

    STORAGE Http(READ SCOPE QUERIES AND PROJECTORS);

    STORAGE SendMail(INSERT SCOPE PROJECTORS);

    STORAGE CmdResult(INSERT SCOPE COMMANDS);

)