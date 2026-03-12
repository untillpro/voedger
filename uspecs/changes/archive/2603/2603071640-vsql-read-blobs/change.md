---
registered_at: 2026-02-25T12:59:27Z
change_id: 2602251259-vsql-read-blobs
baseline: 903b51ddca63185b348d9d447d7629782681898e
issue_url: https://untill.atlassian.net/browse/AIR-3003
archived_at: 2026-03-07T16:40:06Z
---

# Change request: VSQL BLOB reading via blobinfo() and blobtext()

## Why

VSQL currently has no way to read BLOB (including CLOB) data stored in document fields, making it impossible to inspect blob metadata or retrieve blob content directly through queries.

## What

Invent and implement two new VSQL functions for reading BLOBs from document fields:

New functions syntax:

- `blobinfo(field)` — returns a JSON object with blob metadata: `name`, `mimetype`, `size`, `status`
- `blobtext(field[, startFrom])` — returns blob content: base64-encoded for binary MIME types, plain text otherwise; limited to first 10000 bytes starting from optional `startFrom` offset

Example usage:

- `select blobinfo(Img1), blobtext(Img1) from air.Restaurant.123.air.DocWithBLOBs.456`

Constraints and implementation:

- `blobinfo()` and `blobtext()` are only allowed when a `docID` is specified or the document is a singleton (WHERE clause is not allowed)
- Use `blobprocessor.IRequestHandler` from QP for the implementation
