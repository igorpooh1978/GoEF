@LINQ-OBJECTS-001
Feature: Enumerate typed local values

  @LINQ-OBJECTS-001-S01
  Scenario: A filter consumes only the necessary prefix
    Given a counting iterator yielding 1, 2, 3 and 4
    When I construct a filter for even values
    Then the iterator has consumed zero values
    When I request its first result
    Then the result is 2 and exactly two values have been consumed

  @LINQ-OBJECTS-001-S02
  Scenario: A channel-backed source retains its single-use behavior
    Given a closed channel containing two distinct values
    When I consume the adapted sequence twice
    Then the first enumeration sees both values in send order
    And the second enumeration sees zero values

  @LINQ-OBJECTS-001-S03
  Scenario: Equal sort keys retain the input order
    Given IDs 9, 4 and 7 with sort keys B, A and B
    When I order by the sort key in ascending order
    Then the resulting IDs in order are 4, 9 and 7

  @LINQ-OBJECTS-001-S04
  Scenario: Single distinguishes a present zero from an ambiguous result
    Given empty, one-zero and two-zero sequences
    When I request the single integer from each sequence
    Then the outcomes are missing, present zero and multiple elements respectively

  @LINQ-OBJECTS-001-S05
  Scenario: Filtering and projection remain deferred when composed
    Given a replayable counting iterator yielding 1, 2, 3 and 4 with a cleanup callback
    When I filter for even values and project each match to its decimal string followed by !
    Then the iterator, predicate and selector are idle
    When I stop after the first result
    Then the result is "2!" and the iterator has consumed 1 and 2
    And the predicate has seen 1 and 2 and the selector has seen only 2
    And the iterator has released its resources once
    When I enumerate the pipeline again to completion
    Then the results are "2!" and "4!" in that order
    And the iterator and predicate have seen all four values during the replay
    And the selector has seen 2 and 4 during the replay
    And the iterator has released its resources twice in total

  @LINQ-OBJECTS-001-S06
  Scenario: Counting projected matches consumes the entire sequence
    Given a replayable counting iterator yielding 1, 2, 4, 2 and 3 with a cleanup callback
    When I filter for even values and project each match to the same one-element slice
    Then the iterator, predicate and selector are idle
    When I count the projected sequence
    Then the count is 3
    And the iterator and predicate have seen 1, 2, 4, 2 and 3 in order
    And the selector has seen 2, 4 and 2 in order
    And the iterator has released its resources once
    When I count the projected sequence again
    Then the count and traversal are the same
    And the iterator has released its resources twice in total

  @LINQ-OBJECTS-001-S07
  Scenario: Indexed filtering and projection count their own inputs
    Given a replayable sequence with values rejected by an indexed filter
    When I project each accepted value with its position in the filtered sequence
    Then the filter positions include rejected values and the projection positions count accepted values
    And an early consumer stop reaches the source
    And both operators restart their positions at zero on replay

  @LINQ-OBJECTS-001-S08
  Scenario: Quantifiers finish at the first conclusive result
    Given a lazy filter and projection over an iterator that records reads and cleanup
    When I ask whether there is a match, a match satisfying a predicate, or all matches satisfy a predicate
    Then each query stops at its first conclusive result and closes the source before returning
    And an empty sequence has no match and satisfies every universal predicate

  @LINQ-OBJECTS-001-S09
  Scenario: A lazy window consumes only its skipped prefix and requested values
    Given a replayable iterator that records reads and cleanup
    When I skip a fixed prefix and take a limited window
    Then only the prefix and requested window are read on each replay
    And taking zero leaves the iterator unopened

  @LINQ-OBJECTS-001-S10
  Scenario: Composition starts only demanded sequence segments
    Given two replayable sequences in encounter order and an additional value
    When I concatenate the sequences or append or prepend the value
    Then enumeration preserves the order and duplicates
    And a consumer stopping in an earlier segment leaves a later source unopened
