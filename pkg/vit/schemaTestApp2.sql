-- Copyright (c) 2020-present unTill Pro, Ltd.
-- @author Denis Gribanov

APPLICATION app2();

ALTERABLE WORKSPACE test_wsWS (
	DESCRIPTOR test_ws (
		IntFld int32 NOT NULL,
		StrFld varchar
	);

	TABLE doc1 INHERITS CDoc();

	EXTENSION ENGINE BUILTIN (
		COMMAND testCmd();
	);
);
