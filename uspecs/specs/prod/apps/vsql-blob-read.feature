Feature: VSQL BLOB reading

  VADeveloper reads BLOB metadata and content from document fields using VSQL functions

  Scenario Outline: VADeveloper reads blob metadata with blobinfo()
    Given document "<doc>" with blob field "<field>" containing a completed blob
    When VADeveloper executes "select blobinfo(<field>) from <source>"
    Then result contains JSON with keys "name", "mimetype", "size"

    Examples:
      | doc                    | field | source                                        |
      | air.DocWithBLOBs       | Img1  | air.Restaurant.123.air.DocWithBLOBs.456       |
      | air.SingletonWithBLOBs | Logo  | air.Restaurant.123.air.SingletonWithBLOBs     |

  Scenario: VADeveloper reads text blob content with blobtext()
    Given document with blob field "Notes" containing text/plain content "hello world"
    When VADeveloper executes "select blobtext(Notes) from air.Restaurant.123.air.DocWithBLOBs.456"
    Then result contains plain text "hello world"

  Scenario: VADeveloper reads binary blob content with blobtext()
    Given document with blob field "Img1" containing application/x-binary content
    When VADeveloper executes "select blobtext(Img1) from air.Restaurant.123.air.DocWithBLOBs.456"
    Then result contains base64-encoded blob content

  Scenario: VADeveloper reads blob content with startFrom offset
    Given document with blob field "BigText" containing 20000 bytes of text/plain content
    When VADeveloper executes "select blobtext(BigText, 5000) from air.Restaurant.123.air.DocWithBLOBs.456"
    Then result contains at most 10000 bytes starting from byte 5000

  Scenario: VADeveloper reads both blobinfo and blobtext together
    Given document with blob field "Img1" containing a completed blob
    When VADeveloper executes "select blobinfo(Img1), blobtext(Img1) from air.Restaurant.123.air.DocWithBLOBs.456"
    Then result contains both blob metadata JSON and blob content

  Rule: Edge cases

    Scenario: Blob functions rejected without docID on non-singleton
      Given "air.DocWithBLOBs" is not a singleton
      When VADeveloper executes "select blobinfo(Img1) from air.Restaurant.123.air.DocWithBLOBs"
      Then error "At least one record ID must be provided" is returned

    Scenario: Blob functions rejected with WHERE clause
      When VADeveloper executes "select blobinfo(Img1) from air.Restaurant.123.air.DocWithBLOBs.456 where id = 456"
      Then error "WHERE clause is not allowed with blob functions" is returned

    Scenario: Blob function with non-existent field
      When VADeveloper executes "select blobinfo(NonExistent) from air.Restaurant.123.air.DocWithBLOBs.456"
      Then error indicating field not found is returned

    Scenario: Duplicate blobinfo call on same field rejected
      When VADeveloper executes "select blobinfo(Img1), blobinfo(Img1) from air.Restaurant.123.air.DocWithBLOBs.456"
      Then error "field \"blobinfo(Img1)\" is selected more than once" is returned

    Scenario: Duplicate blobtext call on same field rejected
      When VADeveloper executes "select blobtext(Img1), blobtext(Img1) from air.Restaurant.123.air.DocWithBLOBs.456"
      Then error "field \"blobtext(Img1)\" is selected more than once" is returned
