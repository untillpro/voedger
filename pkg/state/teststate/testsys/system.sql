-- Copyright (c) 2020-present unTill Pro, Ltd.

-- note: this schema is for tests only. Voedger sys package uses copy of this schema

ABSTRACT TABLE CRecord();
ABSTRACT TABLE WRecord();
ABSTRACT TABLE ORecord();
ABSTRACT TABLE CDoc INHERITS CRecord();
ABSTRACT TABLE ODoc INHERITS ORecord();
ABSTRACT TABLE WDoc INHERITS WRecord();
ABSTRACT TABLE CSingleton INHERITS CDoc();
ABSTRACT WORKSPACE Workspace(
	EXTENSION ENGINE WASM(
        COMMAND NewWorkspace();
    );
);
ALTERABLE WORKSPACE Profile();

TABLE WorkspaceDescriptor INHERITS CSingleton (
	WSKind qname NOT NULL
);

TYPE Raw(
    Body   varchar(65535)
);

EXTENSION ENGINE BUILTIN (

	STORAGE Record(
		/*
		Key:
			ID int64 // used to identify record by ID
			Singletone QName // used to identify singleton 
		*/
		GET SCOPE(COMMANDS, QUERIES, PROJECTORS),
		GETBATCH SCOPE(COMMANDS, QUERIES, PROJECTORS),
		INSERT SCOPE(COMMANDS),
		UPDATE SCOPE(COMMANDS)
	) ENTITY RECORD;

	-- used to validate projector state/intents declaration
	STORAGE View(
		GET SCOPE(COMMANDS, QUERIES, PROJECTORS),
		GETBATCH SCOPE(COMMANDS, QUERIES, PROJECTORS),
		READ SCOPE(QUERIES, PROJECTORS),
		INSERT SCOPE(PROJECTORS),
		UPDATE SCOPE(PROJECTORS)
	) ENTITY VIEW;

	STORAGE WLog(
		/*
		Key:
			Offset int64
			Count int64 (used for Read operation only)
		Value
			RegisteredAt int64
			SyncedAt int64
			DeviceID int64
			Offset int64
			Synced bool
			QName qname
			CUDs []value {
				IsNew bool
				...CUD fields...
			}
		*/
		GET SCOPE(COMMANDS, QUERIES, PROJECTORS),
		READ SCOPE(QUERIES, PROJECTORS)
	);

	STORAGE AppSecret(
		/*
		Key:
			Secret text
		Value:
			Content text
		*/
		GET SCOPE(COMMANDS, QUERIES, PROJECTORS)
	);

	STORAGE RequestSubject(
		/*
		Key: empty
		Value:
			ProfileWSID int64
			Kind int32
			Name text
			Token texts
		*/
		GET SCOPE(COMMANDS, QUERIES)
	);

	STORAGE Http(
		/*
		Key:
			Method text
			Url text
			Body []byte
			HTTPClientTimeoutMilliseconds int64
			Header text (can be called multiple times)
		Value:
			StatusCode int32
			Body []byte
			Header text (headers combined)

		*/
		READ SCOPE(QUERIES, PROJECTORS)
	);

	STORAGE FederationCommand(
		/*
		Key:
			Owner text (optional, default is current app owner)
			AppName text (optional, default is current app name)
			WSID int64 (optional, default is current workspace)
			Token text (optional, default is system token)
			Command qname
			Body text
		Value:
			StatusCode int32
			NewIDs value {
				rawID1: int64
				rawID2: int64
				...
			}
			Result: value // command result
		*/		
		GET SCOPE(QUERIES, PROJECTORS)
	);

	STORAGE SendMail(
		/*
		Key:
			From text
			To text
			СС text
			BCC text
			Host text - SMTP server
			Port int32 - SMTP server
			Username text - SMTP server
			Password text - SMTP server
			Subject text
			Body text
			
		*/
		INSERT SCOPE(PROJECTORS)
	);

	STORAGE Result(
		/*
		Key: empty
		ValueBuilder: depends on the result of the Command or Query
		*/
		INSERT SCOPE(COMMANDS, QUERIES)
	);

	STORAGE Response(
		/*
		Key: empty
		ValueBuilder:
			StatusCode int32
			ErrorMessage text
		*/
		INSERT SCOPE(COMMANDS, QUERIES)
	);

	STORAGE Event(
		/*
		Key: empty
		Value
			WLogOffset int64
			Workspace int64			
			RegisteredAt int64
			SyncedAt int64
			DeviceID int64
			Offset int64
			Synced bool
			QName qname
			Error value {
				ErrStr text
				ValidEvent bool
				QNameFromParams qname				
			}
			ArgumentObject value
			CUDs []value {
				IsNew bool
				...CUD fields...
			}
		*/
		GET SCOPE(PROJECTORS)
	);

	STORAGE CommandContext(
		/*
		Key: empty
		Value
			Workspace int64
			WLogOffset int64
			ArgumentObject value
			ArgumentUnloggedObject value
		*/
		GET SCOPE(COMMANDS)
	);

	STORAGE QueryContext(
		/*
		Key: empty
		Value
			Workspace int64
			WLogOffset int64
			ArgumentObject value
		*/
		GET SCOPE(QUERIES)
	);
)
