Feature: VSQL view reading

  VADeveloper reads view records via VSQL with WHERE filters on key fields of any supported integer kind

  Scenario Outline: VADeveloper filters view by key field of integer kind
    Given view "<view>" with key field "<field>" of kind "<kind>"
    When VADeveloper executes "select * from <view> where <where>"
    Then matching view records are returned

    Examples:
      | view             | field | kind  | where                                              |
      | air.FdmLog       | Month | int8  | Year=2025 and Month=1 and Day=1                    |
      | air.FdmLog       | Year  | int16 | Year=2025 and Month=1 and Day=1                    |
      | app1pkg.DailyIdx | Year  | int32 | Year=2025                                          |
      | sys.plog         | Offset| int64 | Offset >= 1                                        |

  Rule: Edge cases

    Scenario: Partition key field omitted in WHERE
      Given view "air.FdmLog" with partition key "(Year, Month, Day)"
      When VADeveloper executes "select * from air.FdmLog where Year=2025 and Month=1"
      Then error indicating partition key field is required is returned
